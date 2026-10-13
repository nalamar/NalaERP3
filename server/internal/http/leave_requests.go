package apihttp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// HTTP-Anbindung der Urlaubsantraege (ADR 0025, Backlog F.4.6).
//
// Rechte nach dem Muster der Zeiterfassung:
//
//   - leave_requests.self: eigene Antraege stellen und lesen - fuer den
//     Mitarbeiter, der ueber hr_employees.user_id mit dem Login verknuepft
//     ist.
//   - leave_requests.read: Antraege aller Mitarbeiter lesen.
//   - leave_requests.decide: genehmigen und ablehnen - und fuer einen
//     anderen Mitarbeiter einen Antrag stellen (die Personalabteilung
//     erfasst z. B. eine telefonische Krankmeldung).
//
// Wie bei der Zeiterfassung tragen die Routen kein requirePermission; die
// Pruefung haengt davon ab, wessen Antrag betroffen ist.

const (
	permLeaveRequestsSelf   = "leave_requests.self"
	permLeaveRequestsRead   = "leave_requests.read"
	permLeaveRequestsDecide = "leave_requests.decide"

	maxLeaveRequestPageSize = 500
	leaveDateLayout         = "2006-01-02"
)

func parseLeaveRequestID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Urlaubsantrag-ID")
		return uuid.Nil, false
	}
	return id, true
}

func mountLeaveRequestRoutes(protected chi.Router, svc *hr.LeaveService, resolver employeeForUser) {
	protected.Route("/leave-requests", func(r chi.Router) {
		// Antrag stellen. Ohne employee_id fuer den eigenen Mitarbeiter.
		//
		// Das Eingabe-Struct kennt bewusst weder status noch approver_id
		// noch decided_*: was der Aufrufer dazu mitschickt, wird gar nicht
		// erst gelesen. Ein neuer Antrag ist immer offen.
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				EmployeeID *uuid.UUID `json:"employee_id"`
				Typ        string     `json:"typ"`
				StartDate  string     `json:"start_date"`
				EndDate    string     `json:"end_date"`
				Days       float64    `json:"days"`
				Reason     string     `json:"reason"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			lr := hr.LeaveRequest{Typ: in.Typ, Days: in.Days, Reason: in.Reason}
			for _, d := range []struct {
				name, value string
				dst         *time.Time
			}{{"start_date", in.StartDate, &lr.StartDate}, {"end_date", in.EndDate, &lr.EndDate}} {
				if d.value == "" {
					continue // fehlendes Datum meldet der Service
				}
				parsed, err := time.Parse(leaveDateLayout, d.value)
				if err != nil {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültiges Datum in "+d.name+" (erwartet JJJJ-MM-TT)")
					return
				}
				*d.dst = parsed
			}
			employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, in.EmployeeID, permLeaveRequestsDecide, permLeaveRequestsSelf)
			if !ok {
				return
			}
			lr.EmployeeID = employeeID
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Create(req.Context(), lr, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Liste. Mit leave_requests.read und ohne employee_id: alle
		// Antraege des Mandanten (die Sicht der Personalabteilung). Sonst
		// die eines Mitarbeiters - des genannten oder des eigenen.
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			var requested *uuid.UUID
			if v := q.Get("employee_id"); v != "" {
				id, err := uuid.Parse(v)
				if err != nil {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige employee_id")
					return
				}
				requested = &id
			}
			limit, _ := strconv.Atoi(q.Get("limit"))
			offset, _ := strconv.Atoi(q.Get("offset"))
			if limit > maxLeaveRequestPageSize {
				limit = maxLeaveRequestPageSize
			}
			if offset < 0 {
				offset = 0
			}
			var filter *uuid.UUID
			if requested != nil || !hasPermission(req, permLeaveRequestsRead) {
				employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, requested, permLeaveRequestsRead, permLeaveRequestsSelf)
				if !ok {
					return
				}
				filter = &employeeID
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), filter, limit, offset, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseLeaveRequestID(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Get(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			if _, ok := resolveOwnOrPermittedEmployee(w, req, resolver, &out.EmployeeID, permLeaveRequestsRead, permLeaveRequestsSelf); !ok {
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Entscheiden: {"approve": true} genehmigt, {"approve": false}
		// lehnt ab. approve ist Pflicht - ein vergessenes Feld darf nicht
		// als Ablehnung gelten.
		//
		// Festgehalten wird der Login, der entscheidet; hat er einen
		// verknuepften Mitarbeiter, wird der als approver_id mitgefuehrt.
		r.With(requirePermission(permLeaveRequestsDecide)).Post("/{id}/decide", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseLeaveRequestID(w, req)
			if !ok {
				return
			}
			var in struct {
				Approve *bool `json:"approve"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			if in.Approve == nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "approve erforderlich")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			actor := actorUserIDFromContext(req.Context())
			var approver *uuid.UUID
			own, linked, err := resolver.EmployeeIDForUser(req.Context(), actor, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			if linked {
				approver = &own
			}
			out, err := svc.Decide(req.Context(), id, *in.Approve, actor, approver, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
