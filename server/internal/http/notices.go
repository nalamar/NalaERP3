package apihttp

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/notices"
)

// mountNoticeRoutes haengt GET /notices ein (Backlog F.10.4.1): die
// Hinweise fuer den angemeldeten Benutzer. Kein eigenes Recht - jeder
// angemeldete Benutzer sieht seine Hinweise; welche Bereiche dazugehoeren,
// entscheiden die Rechte, die er ohnehin hat (notices.Source).
func mountNoticeRoutes(protected chi.Router, svc *notices.Service, resolver employeeForUser) {
	protected.Get("/notices", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		userID := actorUserIDFromContext(req.Context())
		r := notices.Recipient{
			UserID:    userID,
			CompanyID: companyID,
			Can:       func(p string) bool { return hasPermission(req, p) },
		}
		if userID != "" {
			own, linked, err := resolver.EmployeeIDForUser(req.Context(), userID, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			if linked {
				r.EmployeeID = &own
			}
		}
		out, err := svc.For(req.Context(), r, hr.WorkDateOf(time.Now()))
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}
