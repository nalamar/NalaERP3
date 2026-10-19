package hr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Employee struct {
	ID          uuid.UUID  `json:"id"`
	PersonalNr  *string    `json:"personalnummer,omitempty"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	HireDate    *time.Time `json:"hire_date,omitempty"`
	Termination *time.Time `json:"termination_date,omitempty"`
	Role        string     `json:"role"`
	TeamID      *uuid.UUID `json:"team_id,omitempty"`
	Location    string     `json:"location"`
	CostCenter  string     `json:"cost_center"`
	Active      bool       `json:"active"`
	// UserID ist der verknuepfte Login-Benutzer (siehe SetUser). Er wird
	// nur gelesen; gesetzt wird er ausschliesslich ueber SetUser.
	UserID *string `json:"user_id,omitempty"`
	// Bundesland ueberschreibt das des Mandanten (ADR 0029); nil = es gilt
	// der Mandant.
	Bundesland *string `json:"bundesland"`
	// Arbeitstage nach ISO 8601 (1 = Montag ... 7 = Sonntag), sortiert.
	// Daraus zaehlen Urlaubsantraege ihre Tage.
	Arbeitstage []int `json:"arbeitstage"`
	// UrlaubstageJahr ist ein ausgehandelter Jahresanspruch; nil = Vorgabe
	// (4 Tage je woechentlichem Arbeitstag, ADR 0029).
	UrlaubstageJahr *float64 `json:"urlaubstage_jahr"`
}

// EmployeeInput sind die Angaben fuer die Anlage eines Mitarbeiters.
//
// Active ist ein Zeiger, weil "nicht angegeben" und "false" verschiedene
// Dinge sind: ein neuer Mitarbeiter ist AKTIV, solange nicht ausdruecklich
// false angegeben wird (ADR 0025, Entscheidung des Nutzers vom
// 2026-10-13). Mit einem blossen bool waere jeder ohne die Angabe
// angelegte Mitarbeiter inaktiv gewesen - so war es bis F.4.3.
type EmployeeInput struct {
	PersonalNr  *string    `json:"personalnummer,omitempty"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	HireDate    *time.Time `json:"hire_date,omitempty"`
	Termination *time.Time `json:"termination_date,omitempty"`
	Role        string     `json:"role"`
	TeamID      *uuid.UUID `json:"team_id,omitempty"`
	Location    string     `json:"location"`
	CostCenter  string     `json:"cost_center"`
	Active      *bool      `json:"active,omitempty"`
	Bundesland  *string    `json:"bundesland,omitempty"`
	// Arbeitstage nil = Montag bis Freitag.
	Arbeitstage     []int    `json:"arbeitstage,omitempty"`
	UrlaubstageJahr *float64 `json:"urlaubstage_jahr,omitempty"`
}

// Namen der UNIQUE-Indizes, deren Verletzung in eine fachliche Meldung
// uebersetzt wird (087 bzw. 086). Die Uebersetzung statt einer
// Vorabpruefung ist Absicht: sie ist auch dann richtig, wenn zwei Aufrufe
// gleichzeitig kommen, und der rohe Postgres-Fehler - der im DETAIL die
// Personalnummer nennt - erreicht so weder den Aufrufer noch das Log.
const (
	employeePersonalNrIndex = "idx_hr_employees_company_personalnummer"
	employeeUserIDIndex     = "idx_hr_employees_user_id"
)

func isUniqueViolation(err error, index string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == index
}

type EmployeeService struct{ pg *pgxpool.Pool }

func NewEmployeeService(pg *pgxpool.Pool) *EmployeeService { return &EmployeeService{pg: pg} }

type Team struct {
	ID   uuid.UUID  `json:"id"`
	Name string     `json:"name"`
	Lead *uuid.UUID `json:"lead_employee_id,omitempty"`
}

func (s *EmployeeService) ListTeams(ctx context.Context, companyID string) ([]Team, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, name, lead_employee_id FROM hr_teams WHERE company_id=$1 ORDER BY name`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Team, 0)
	for rows.Next() {
		var t Team
		var lead *uuid.UUID
		if err := rows.Scan(&t.ID, &t.Name, &lead); err != nil {
			return nil, err
		}
		t.Lead = lead
		out = append(out, t)
	}
	return out, nil
}

func (s *EmployeeService) List(ctx context.Context, limit, offset int, companyID string) ([]Employee, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pg.Query(ctx, `SELECT id, personalnummer, first_name, last_name, COALESCE(email,''), COALESCE(phone,''), hire_date, termination_date, COALESCE(role,''), team_id, COALESCE(location,''), COALESCE(cost_center,''), active, user_id, bundesland, arbeitstage, urlaubstage_jahr::float8 FROM hr_employees WHERE company_id=$1 ORDER BY last_name, first_name LIMIT $2 OFFSET $3`, companyID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Employee, 0)
	for rows.Next() {
		var e Employee
		var hire, term *time.Time
		var teamID *uuid.UUID
		var pn *string
		var days []int16
		if err := rows.Scan(&e.ID, &pn, &e.FirstName, &e.LastName, &e.Email, &e.Phone, &hire, &term, &e.Role, &teamID, &e.Location, &e.CostCenter, &e.Active, &e.UserID, &e.Bundesland, &days, &e.UrlaubstageJahr); err != nil {
			return nil, err
		}
		e.Arbeitstage = int16sToInts(days)
		e.HireDate = hire
		e.Termination = term
		e.TeamID = teamID
		e.PersonalNr = pn
		out = append(out, e)
	}
	return out, rows.Err()
}

// teamBelongsToCompany prueft, dass ein Team zum Mandanten gehoert. Der
// Fremdschluessel allein genuegt nicht: er wuerde auch ein Team eines
// anderen Mandanten akzeptieren.
func (s *EmployeeService) teamBelongsToCompany(ctx context.Context, teamID uuid.UUID, companyID string) error {
	var owned bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_teams WHERE id=$1 AND company_id=$2)`, teamID, companyID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return errors.New("Team nicht gefunden")
	}
	return nil
}

func (s *EmployeeService) Create(ctx context.Context, in EmployeeInput, companyID string) (*Employee, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.FirstName == "" || in.LastName == "" {
		return nil, errors.New("Vor- und Nachname erforderlich")
	}
	e := Employee{
		ID:          uuid.New(),
		FirstName:   in.FirstName,
		LastName:    in.LastName,
		Email:       in.Email,
		Phone:       in.Phone,
		HireDate:    in.HireDate,
		Termination: in.Termination,
		Role:        in.Role,
		TeamID:      in.TeamID,
		Location:    in.Location,
		CostCenter:  in.CostCenter,
		Active:      in.Active == nil || *in.Active,
	}
	// Eine leere Personalnummer ist "keine Personalnummer". Als leerer
	// String gespeichert waere sie ein Wert - und der zweite Mitarbeiter
	// ohne Nummer wuerde an der Eindeutigkeit scheitern.
	if in.PersonalNr != nil {
		if pn := strings.TrimSpace(*in.PersonalNr); pn != "" {
			e.PersonalNr = &pn
		}
	}
	land, err := normalizeEmployeeBundesland(in.Bundesland)
	if err != nil {
		return nil, err
	}
	e.Bundesland = land
	if e.Arbeitstage, err = normalizeArbeitstage(in.Arbeitstage); err != nil {
		return nil, err
	}
	if err := validateEmployeeLeaveDays(in.UrlaubstageJahr, e.Arbeitstage); err != nil {
		return nil, err
	}
	e.UrlaubstageJahr = in.UrlaubstageJahr
	if e.TeamID != nil {
		if err := s.teamBelongsToCompany(ctx, *e.TeamID, companyID); err != nil {
			return nil, err
		}
	}
	_, err = s.pg.Exec(ctx, `INSERT INTO hr_employees (id, personalnummer, first_name, last_name, email, phone, hire_date, termination_date, role, team_id, location, cost_center, active, company_id, bundesland, arbeitstage, urlaubstage_jahr)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		e.ID, e.PersonalNr, e.FirstName, e.LastName, e.Email, e.Phone, e.HireDate, e.Termination, e.Role, e.TeamID, e.Location, e.CostCenter, e.Active, companyID, e.Bundesland, intsToInt16s(e.Arbeitstage), e.UrlaubstageJahr)
	if err != nil {
		if isUniqueViolation(err, employeePersonalNrIndex) {
			return nil, errors.New("Personalnummer existiert bereits")
		}
		return nil, err
	}
	return &e, nil
}

func (s *EmployeeService) Get(ctx context.Context, id uuid.UUID, companyID string) (*Employee, error) {
	var e Employee
	var hire, term *time.Time
	var teamID *uuid.UUID
	var pn *string
	var days []int16
	err := s.pg.QueryRow(ctx, `SELECT id, personalnummer, first_name, last_name, COALESCE(email,''), COALESCE(phone,''), hire_date, termination_date, COALESCE(role,''), team_id, COALESCE(location,''), COALESCE(cost_center,''), active, user_id, bundesland, arbeitstage, urlaubstage_jahr::float8 FROM hr_employees WHERE id=$1 AND company_id=$2`, id, companyID).
		Scan(&e.ID, &pn, &e.FirstName, &e.LastName, &e.Email, &e.Phone, &hire, &term, &e.Role, &teamID, &e.Location, &e.CostCenter, &e.Active, &e.UserID, &e.Bundesland, &days, &e.UrlaubstageJahr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Mitarbeiter nicht gefunden")
		}
		return nil, err
	}
	e.HireDate = hire
	e.Termination = term
	e.TeamID = teamID
	e.PersonalNr = pn
	e.Arbeitstage = int16sToInts(days)
	return &e, nil
}

// employeeUpdatableFields sind die einzigen Patch-Keys, die Update() in die
// SQL-SET-Klausel uebernimmt. Frueher fiel jeder andere Key (z.B. ein
// Tippfehler wie "activ" statt "active") stillschweigend durch einen
// switch ohne default-Fall und wurde ignoriert - der Aufruf kehrte ohne
// Fehler zurueck, obwohl nichts geaendert wurde (Backlog 0.10). Jetzt wird
// jeder unbekannte Key VOR jedem DB-Zugriff mit einem Fehler abgelehnt.
var employeeUpdatableFields = map[string]bool{
	"first_name": true, "last_name": true, "email": true, "phone": true,
	"role": true, "location": true, "cost_center": true, "active": true, "team_id": true,
	"bundesland": true, "arbeitstage": true, "urlaubstage_jahr": true,
}

func (s *EmployeeService) Update(ctx context.Context, id uuid.UUID, patch map[string]any, companyID string) error {
	if len(patch) == 0 {
		return nil
	}
	for k := range patch {
		if !employeeUpdatableFields[k] {
			return fmt.Errorf("unbekanntes Feld: %s", k)
		}
	}
	// Typen pruefen, bevor die Werte an Postgres gehen: ein Patch kommt
	// als JSON und kann fuer jedes Feld jeden Typ tragen. Ungeprueft kaeme
	// z. B. "active": "ja" als roher Datenbankfehler zurueck.
	for k, v := range patch {
		switch k {
		case "active":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("Ungültiger Wert für Feld: %s", k)
			}
		case "team_id":
			teamID, err := patchTeamID(v)
			if err != nil {
				return err
			}
			if teamID != nil {
				if err := s.teamBelongsToCompany(ctx, *teamID, companyID); err != nil {
					return err
				}
			}
			patch[k] = teamID
		case "bundesland":
			land, err := patchBundesland(v)
			if err != nil {
				return err
			}
			patch[k] = land
		case "arbeitstage":
			days, err := patchArbeitstage(v)
			if err != nil {
				return err
			}
			patch[k] = intsToInt16s(days)
		case "urlaubstage_jahr":
			switch t := v.(type) {
			case nil:
			case float64:
				patch[k] = &t
			default:
				return fmt.Errorf("Ungültiger Wert für Feld: %s", k)
			}
		default:
			if _, ok := v.(string); !ok {
				return fmt.Errorf("Ungültiger Wert für Feld: %s", k)
			}
		}
	}
	if err := s.checkLeaveDaysAfterPatch(ctx, id, patch, companyID); err != nil {
		return err
	}
	sets := make([]string, 0, len(patch))
	args := make([]any, 0, len(patch)+2)
	i := 1
	for k, v := range patch {
		sets = append(sets, fmt.Sprintf("%s=$%d", k, i))
		args = append(args, v)
		i++
	}
	args = append(args, id, companyID)
	ct, err := s.pg.Exec(ctx, `UPDATE hr_employees SET `+strings.Join(sets, ", ")+fmt.Sprintf(" WHERE id=$%d AND company_id=$%d", len(args)-1, len(args)), args...)
	if err != nil {
		return err
	}
	// Ohne diese Pruefung meldete eine Aenderung an einem unbekannten oder
	// fremden Mitarbeiter Erfolg, obwohl nichts geschrieben wurde.
	if ct.RowsAffected() == 0 {
		return errors.New("Mitarbeiter nicht gefunden")
	}
	return nil
}

// patchTeamID liest team_id aus einem Patch: nil loest die Zuordnung, sonst
// eine UUID als Text (so kommt sie aus JSON) oder als uuid.UUID.
func patchTeamID(v any) (*uuid.UUID, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case uuid.UUID:
		return &t, nil
	case string:
		id, err := uuid.Parse(t)
		if err != nil {
			return nil, errors.New("Ungültiger Wert für Feld: team_id")
		}
		return &id, nil
	default:
		return nil, errors.New("Ungültiger Wert für Feld: team_id")
	}
}

// SetUser verknuepft einen Mitarbeiter mit einem Login-Benutzer oder loest
// die Verknuepfung (userID leer). Die Verknuepfung entscheidet, wessen
// Zeiten ein Login mit time_entries.self bucht - deshalb eine eigene, enge
// Funktion mit eigenen Pruefungen und kein Feld im generischen Patch von
// Update (ADR 0025, analog projects.SetKostenstelle).
func (s *EmployeeService) SetUser(ctx context.Context, id uuid.UUID, userID, companyID string) error {
	if strings.TrimSpace(companyID) == "" {
		return errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return errors.New("Mitarbeiter-ID erforderlich")
	}
	var link *string
	if userID = strings.TrimSpace(userID); userID != "" {
		var userOwned bool
		if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND company_id=$2)`, userID, companyID).Scan(&userOwned); err != nil {
			return err
		}
		if !userOwned {
			return errors.New("Benutzer nicht gefunden")
		}
		link = &userID
	}
	ct, err := s.pg.Exec(ctx, `UPDATE hr_employees SET user_id=$1 WHERE id=$2 AND company_id=$3`, link, id, companyID)
	if err != nil {
		if isUniqueViolation(err, employeeUserIDIndex) {
			return errors.New("Benutzer ist bereits mit einem anderen Mitarbeiter verknüpft")
		}
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("Mitarbeiter nicht gefunden")
	}
	return nil
}

// LoginUser ist ein Login des Mandanten, wie ihn die Personalabteilung zum
// Verknuepfen mit einem Mitarbeiter braucht (Backlog F.7.2.1). Bewusst
// schmal: keine Rollen, keine Rechte, kein Anmeldestatus - die
// Benutzerverwaltung (users.manage) bleibt der Administration vorbehalten.
type LoginUser struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	// EmployeeID ist der Mitarbeiter, mit dem der Login bereits verknuepft
	// ist; nil = frei.
	EmployeeID *uuid.UUID `json:"employee_id,omitempty"`
}

// ListLoginUsers liefert die AKTIVEN Logins des Mandanten, nach E-Mail
// sortiert, jeweils mit dem bereits verknuepften Mitarbeiter. Deaktivierte
// Logins erscheinen nicht - mit ihnen kann niemand stempeln. Gesperrte
// (zu viele Fehlversuche) erscheinen: die Sperre ist voruebergehend.
func (s *EmployeeService) ListLoginUsers(ctx context.Context, companyID string) ([]LoginUser, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	rows, err := s.pg.Query(ctx, `
		SELECT u.id, u.email,
		       COALESCE(NULLIF(btrim(u.display_name), ''), btrim(u.first_name || ' ' || u.last_name)),
		       e.id
		  FROM users u
		  LEFT JOIN hr_employees e ON e.user_id = u.id AND e.company_id = $1
		 WHERE u.company_id = $1 AND u.is_active
		 ORDER BY lower(u.email), u.id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LoginUser, 0)
	for rows.Next() {
		var u LoginUser
		if err := rows.Scan(&u.UserID, &u.Email, &u.Name, &u.EmployeeID); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type LeaveRequest struct {
	ID         uuid.UUID  `json:"id"`
	EmployeeID uuid.UUID  `json:"employee_id"`
	Typ        string     `json:"typ"`
	Status     string     `json:"status"`
	StartDate  time.Time  `json:"start_date"`
	EndDate    time.Time  `json:"end_date"`
	Days       float64    `json:"days"`
	Reason     string     `json:"reason"`
	ApproverID *uuid.UUID `json:"approver_id,omitempty"`
	// DecidedByUserID ist der Login, der entschieden hat (ADR 0025).
	// ApproverID ist dessen Mitarbeiterdatensatz, sofern er einen hat.
	DecidedByUserID *string    `json:"decided_by_user_id,omitempty"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	// Zurueckziehen und Storno (F.8.4.1, Migration 093).
	WithdrawnAt       *time.Time `json:"withdrawn_at,omitempty"`
	StorniertAt       *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID *string    `json:"storniert_by_user_id,omitempty"`
	StornoReason      string     `json:"storno_reason,omitempty"`
}

// leaveTypes sind die zulaessigen Antragstypen (020_hr_base.sql).
var leaveTypes = map[string]bool{"vacation": true, "sick": true, "unpaid": true, "other": true}

const leaveRequestColumns = `id, employee_id, typ, status, start_date, end_date, days, COALESCE(reason,''), approver_id, decided_by_user_id, decided_at, created_at, withdrawn_at, storniert_at, storniert_by_user_id, COALESCE(storno_reason,'')`

func scanLeaveRequest(row pgx.Row) (*LeaveRequest, error) {
	var lr LeaveRequest
	if err := row.Scan(&lr.ID, &lr.EmployeeID, &lr.Typ, &lr.Status, &lr.StartDate, &lr.EndDate, &lr.Days, &lr.Reason, &lr.ApproverID, &lr.DecidedByUserID, &lr.DecidedAt, &lr.CreatedAt, &lr.WithdrawnAt, &lr.StorniertAt, &lr.StorniertByUserID, &lr.StornoReason); err != nil {
		return nil, err
	}
	return &lr, nil
}

type LeaveService struct{ pg *pgxpool.Pool }

func NewLeaveService(pg *pgxpool.Pool) *LeaveService { return &LeaveService{pg: pg} }

func (s *LeaveService) Create(ctx context.Context, lr LeaveRequest, companyID string) (*LeaveRequest, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if lr.EmployeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	if lr.StartDate.IsZero() || lr.EndDate.IsZero() {
		return nil, errors.New("start/end erforderlich")
	}
	if lr.EndDate.Before(lr.StartDate) {
		return nil, errors.New("Enddatum darf nicht vor Startdatum liegen")
	}
	var employeeOwned bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, lr.EmployeeID, companyID).Scan(&employeeOwned); err != nil {
		return nil, err
	}
	if !employeeOwned {
		return nil, errors.New("Mitarbeiter nicht gefunden")
	}
	// Ueberschneidungspruefung (Backlog 0.12): pending/approved Antraege
	// desselben Mitarbeiters mit sich ueberlappendem Zeitraum blockieren
	// einen neuen Antrag. Bereits abgelehnte Antraege (status='rejected')
	// zaehlen bewusst NICHT - ein abgelehnter Antrag soll den Zeitraum
	// nicht dauerhaft blockieren.
	var overlapping bool
	if err := s.pg.QueryRow(ctx, `
        SELECT EXISTS(
            SELECT 1 FROM hr_leave_requests
             WHERE employee_id=$1 AND status IN ('pending','approved')
               AND start_date <= $2 AND end_date >= $3
        )
    `, lr.EmployeeID, lr.EndDate, lr.StartDate).Scan(&overlapping); err != nil {
		return nil, err
	}
	if overlapping {
		return nil, errors.New("Zeitraum überschneidet sich mit einem bestehenden Urlaubsantrag")
	}
	if lr.Typ == "" {
		lr.Typ = "vacation"
	}
	if !leaveTypes[lr.Typ] {
		return nil, errors.New("Ungültiger Antragstyp")
	}
	// Krankheit wird gemeldet, nicht beantragt (ADR 0029, F.8.4.2) - als
	// Krankmeldung ueber AbsenceService.ReportSick.
	if lr.Typ == "sick" {
		return nil, errors.New("Ungültiger Antragstyp: Krankheit wird als Krankmeldung erfasst, nicht beantragt")
	}
	// Ohne Angabe zaehlen die Arbeitstage des Zeitraums (ADR 0029): die
	// Arbeitswoche des Mitarbeiters ohne Feiertage und betriebliche freie
	// Tage - bis F.8.2.3 waren es Kalendertage. Eine Angabe darf kleiner
	// sein (halbe Tage), aber nicht groesser - sonst liesse sich fuer zwei
	// Tage ein beliebiger Wert beantragen. Bestehende Antraege behalten
	// ihre gespeicherte Zahl.
	workDays, err := NewCalendarService(s.pg).EmployeeWorkDays(ctx, lr.EmployeeID, lr.StartDate, lr.EndDate, companyID)
	if err != nil {
		return nil, err
	}
	if workDays == 0 {
		return nil, errors.New("Ungültiger Zeitraum: er enthält keinen Arbeitstag")
	}
	if lr.Days <= 0 {
		lr.Days = float64(workDays)
	}
	if lr.Days > float64(workDays) {
		return nil, errors.New("Tage dürfen die Arbeitstage des Zeitraums nicht überschreiten")
	}
	lr.ID = uuid.New()
	lr.Status = "pending"
	lr.CreatedAt = time.Now()
	_, err = s.pg.Exec(ctx, `INSERT INTO hr_leave_requests (id, employee_id, typ, status, start_date, end_date, days, reason) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		lr.ID, lr.EmployeeID, lr.Typ, lr.Status, lr.StartDate, lr.EndDate, lr.Days, lr.Reason)
	if err != nil {
		return nil, err
	}
	return &lr, nil
}

// ErrNoDecisionRight: der Login darf diesen Antrag nicht entscheiden oder
// stornieren. Die Routen melden das als 403.
var ErrNoDecisionRight = errors.New("Keine Berechtigung, diesen Urlaubsantrag zu entscheiden")

// LeaveDecisionRight beschreibt, welche Antraege ein Login entscheiden und
// stornieren darf (ADR 0029, Entscheidung des Nutzers vom 2026-10-19:
// "Urlaub genehmigt nur Personalabteilung oder Geschaeftsfuehrer im Moment,
// eine Erweiterung auf Teamleiter aber bitte schon mit vorbereiten").
//
// All: alle Antraege des Mandanten (Recht leave_requests.decide).
//
// Vorbereitung Teamleiter: die Frage wird NUR in mayDecide beantwortet,
// und mayDecide kennt den Antrag. Fuer Teamleiter kommt hier ein Feld
// hinzu (der Mitarbeiter des Logins, gesetzt bei einem eigenen Recht) und
// in mayDecide der Vergleich mit hr_teams.lead_employee_id des Teams des
// Antragstellers - Routen und Aufrufer bleiben unveraendert.
type LeaveDecisionRight struct {
	All bool
}

// mayDecide ist die eine Stelle, an der entschieden wird, ob ein Login
// einen bestimmten Antrag entscheiden oder stornieren darf.
func mayDecide(right LeaveDecisionRight, _ *LeaveRequest) error {
	if right.All {
		return nil
	}
	return ErrNoDecisionRight
}

// Decide genehmigt einen Antrag oder lehnt ihn ab.
//
// Die Entscheidung ist einmalig: entschieden werden kann nur ein Antrag im
// Status pending. Eine Genehmigung legt in derselben Transaktion die
// Abwesenheit an (hr_absences, ADR 0029) - es gibt keinen genehmigten
// Antrag ohne Abwesenheit.
//
// deciderUserID ist der Login, der entscheidet - an ihm haengt der
// Nachweis. approver ist dessen Mitarbeiterdatensatz, falls er einen hat.
func (s *LeaveService) Decide(ctx context.Context, id uuid.UUID, approve bool, right LeaveDecisionRight, deciderUserID string, approver *uuid.UUID, companyID string) (*LeaveRequest, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Urlaubsantrag-ID erforderlich")
	}
	if strings.TrimSpace(deciderUserID) == "" {
		return nil, errors.New("Entscheider erforderlich")
	}
	lr, err := s.Get(ctx, id, companyID)
	if err != nil {
		return nil, err
	}
	if err := mayDecide(right, lr); err != nil {
		return nil, err
	}
	if approver != nil {
		var approverOwned bool
		if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, *approver, companyID).Scan(&approverOwned); err != nil {
			return nil, err
		}
		if !approverOwned {
			return nil, errors.New("Mitarbeiter nicht gefunden")
		}
	}
	status := "rejected"
	if approve {
		status = "approved"
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// Die Statuspruefung steht im UPDATE selbst: zwei gleichzeitige
	// Entscheidungen koennen so nicht beide durchgehen.
	ct, err := tx.Exec(ctx, `UPDATE hr_leave_requests SET status=$2, approver_id=$3, decided_by_user_id=$4, decided_at=now()
		WHERE id=$1 AND status='pending' AND employee_id IN (SELECT id FROM hr_employees WHERE company_id=$5)`, id, status, approver, deciderUserID, companyID)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, errors.New("Urlaubsantrag ist bereits entschieden")
	}
	if approve {
		if _, err := tx.Exec(ctx, `INSERT INTO hr_absences (id, employee_id, typ, start_date, end_date, status, source_request_id)
			SELECT $2, employee_id, typ, start_date, end_date, 'approved', id FROM hr_leave_requests WHERE id=$1`, id, uuid.New()); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// Withdraw zieht einen offenen Antrag zurueck. Wer das darf (der
// Antragsteller selbst oder die Personalabteilung), prueft die Route.
// Ein entschiedener Antrag laesst sich nicht zurueckziehen - ein
// genehmigter wird storniert (Storno).
func (s *LeaveService) Withdraw(ctx context.Context, id uuid.UUID, companyID string) (*LeaveRequest, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if _, err := s.Get(ctx, id, companyID); err != nil {
		return nil, err
	}
	ct, err := s.pg.Exec(ctx, `UPDATE hr_leave_requests SET status='withdrawn', withdrawn_at=now()
		WHERE id=$1 AND status='pending' AND employee_id IN (SELECT id FROM hr_employees WHERE company_id=$2)`, id, companyID)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, errors.New("Urlaubsantrag ist bereits entschieden")
	}
	return s.Get(ctx, id, companyID)
}

// Storno storniert einen genehmigten Antrag mit Begruendung; die
// Abwesenheit wird in derselben Transaktion mitstorniert, die Tage stehen
// damit wieder im Urlaubskonto. Auch vergangene Zeitraeume sind
// stornierbar (Fehlbuchung) - die Begruendung ist Pflicht.
func (s *LeaveService) Storno(ctx context.Context, id uuid.UUID, reason string, right LeaveDecisionRight, actorUserID, companyID string) (*LeaveRequest, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("Begründung erforderlich")
	}
	if strings.TrimSpace(actorUserID) == "" {
		return nil, errors.New("Entscheider erforderlich")
	}
	lr, err := s.Get(ctx, id, companyID)
	if err != nil {
		return nil, err
	}
	if err := mayDecide(right, lr); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM hr_leave_requests WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return nil, err
	}
	switch status {
	case "approved":
	case "storniert":
		return nil, errors.New("Urlaubsantrag ist bereits storniert")
	default:
		return nil, errors.New("Urlaubsantrag ist nicht im Status genehmigt - offene Anträge werden zurückgezogen")
	}
	if _, err := tx.Exec(ctx, `UPDATE hr_leave_requests SET status='storniert', storniert_at=now(), storniert_by_user_id=$2, storno_reason=$3 WHERE id=$1`, id, actorUserID, reason); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE hr_absences SET status='storniert', storniert_at=now(), storniert_by_user_id=$2, storno_reason=$3
		WHERE source_request_id=$1 AND status='approved'`, id, actorUserID, reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

func (s *LeaveService) Get(ctx context.Context, id uuid.UUID, companyID string) (*LeaveRequest, error) {
	lr, err := scanLeaveRequest(s.pg.QueryRow(ctx, `SELECT `+leaveRequestColumns+` FROM hr_leave_requests
		WHERE id=$1 AND employee_id IN (SELECT id FROM hr_employees WHERE company_id=$2)`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Urlaubsantrag nicht gefunden")
		}
		return nil, err
	}
	return lr, nil
}

func (s *LeaveService) List(ctx context.Context, employeeID *uuid.UUID, limit, offset int, companyID string) ([]LeaveRequest, error) {
	if limit <= 0 {
		limit = 50
	}
	args := []any{}
	where := ""
	if employeeID != nil {
		var employeeOwned bool
		if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, *employeeID, companyID).Scan(&employeeOwned); err != nil {
			return nil, err
		}
		if !employeeOwned {
			return nil, errors.New("Mitarbeiter nicht gefunden")
		}
		where = "WHERE employee_id=$1"
		args = append(args, *employeeID)
	} else {
		where = "WHERE employee_id IN (SELECT id FROM hr_employees WHERE company_id=$1)"
		args = append(args, companyID)
	}
	args = append(args, limit, offset)
	rows, err := s.pg.Query(ctx, `SELECT `+leaveRequestColumns+` FROM hr_leave_requests `+where+` ORDER BY created_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LeaveRequest, 0)
	for rows.Next() {
		lr, err := scanLeaveRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *lr)
	}
	return out, rows.Err()
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// checkLeaveDaysAfterPatch prueft den Jahresanspruch gegen die Arbeitstage,
// wie sie nach dem Patch waeren - auch wenn nur die Arbeitstage geaendert
// werden (mehr Arbeitstage heben die Untergrenze). Der CHECK in 092 sichert
// dasselbe in der Datenbank; hier entsteht die verstaendliche Meldung.
func (s *EmployeeService) checkLeaveDaysAfterPatch(ctx context.Context, id uuid.UUID, patch map[string]any, companyID string) error {
	newDays, daysPatched := patch["arbeitstage"]
	newLeave, leavePatched := patch["urlaubstage_jahr"]
	if !daysPatched && !leavePatched {
		return nil
	}
	var leave *float64
	var stored []int16
	err := s.pg.QueryRow(ctx, `SELECT urlaubstage_jahr::float8, arbeitstage FROM hr_employees WHERE id=$1 AND company_id=$2`, id, companyID).Scan(&leave, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("Mitarbeiter nicht gefunden")
	}
	if err != nil {
		return err
	}
	days := int16sToInts(stored)
	if daysPatched {
		days = int16sToInts(newDays.([]int16))
	}
	if leavePatched {
		leave, _ = newLeave.(*float64)
	}
	return validateEmployeeLeaveDays(leave, days)
}
