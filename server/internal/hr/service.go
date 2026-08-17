package hr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
	rows, err := s.pg.Query(ctx, `SELECT id, personalnummer, first_name, last_name, COALESCE(email,''), COALESCE(phone,''), hire_date, termination_date, COALESCE(role,''), team_id, COALESCE(location,''), COALESCE(cost_center,''), active FROM hr_employees WHERE company_id=$1 ORDER BY last_name, first_name LIMIT $2 OFFSET $3`, companyID, limit, offset)
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
		if err := rows.Scan(&e.ID, &pn, &e.FirstName, &e.LastName, &e.Email, &e.Phone, &hire, &term, &e.Role, &teamID, &e.Location, &e.CostCenter, &e.Active); err != nil {
			return nil, err
		}
		e.HireDate = hire
		e.Termination = term
		e.TeamID = teamID
		e.PersonalNr = pn
		out = append(out, e)
	}
	return out, nil
}

func (s *EmployeeService) Create(ctx context.Context, e Employee, companyID string) (*Employee, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if e.FirstName == "" || e.LastName == "" {
		return nil, errors.New("Vor- und Nachname erforderlich")
	}
	id := uuid.New()
	_, err := s.pg.Exec(ctx, `INSERT INTO hr_employees (id, personalnummer, first_name, last_name, email, phone, hire_date, termination_date, role, team_id, location, cost_center, active, company_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		id, e.PersonalNr, e.FirstName, e.LastName, e.Email, e.Phone, e.HireDate, e.Termination, e.Role, e.TeamID, e.Location, e.CostCenter, e.Active, companyID)
	if err != nil {
		return nil, err
	}
	e.ID = id
	return &e, nil
}

func (s *EmployeeService) Get(ctx context.Context, id uuid.UUID, companyID string) (*Employee, error) {
	var e Employee
	var hire, term *time.Time
	var teamID *uuid.UUID
	var pn *string
	err := s.pg.QueryRow(ctx, `SELECT id, personalnummer, first_name, last_name, COALESCE(email,''), COALESCE(phone,''), hire_date, termination_date, COALESCE(role,''), team_id, COALESCE(location,''), COALESCE(cost_center,''), active FROM hr_employees WHERE id=$1 AND company_id=$2`, id, companyID).
		Scan(&e.ID, &pn, &e.FirstName, &e.LastName, &e.Email, &e.Phone, &hire, &term, &e.Role, &teamID, &e.Location, &e.CostCenter, &e.Active)
	if err != nil {
		return nil, err
	}
	e.HireDate = hire
	e.Termination = term
	e.TeamID = teamID
	e.PersonalNr = pn
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
	sets := make([]string, 0, len(patch))
	args := make([]any, 0, len(patch)+2)
	i := 1
	for k, v := range patch {
		sets = append(sets, fmt.Sprintf("%s=$%d", k, i))
		args = append(args, v)
		i++
	}
	args = append(args, id, companyID)
	_, err := s.pg.Exec(ctx, `UPDATE hr_employees SET `+strings.Join(sets, ", ")+fmt.Sprintf(" WHERE id=$%d AND company_id=$%d", len(args)-1, len(args)), args...)
	return err
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
	DecidedAt  *time.Time `json:"decided_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
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
	if lr.Days <= 0 {
		lr.Days = lr.EndDate.Sub(lr.StartDate).Hours()/24 + 1
	}
	lr.ID = uuid.New()
	lr.Status = "pending"
	lr.CreatedAt = time.Now()
	_, err := s.pg.Exec(ctx, `INSERT INTO hr_leave_requests (id, employee_id, typ, status, start_date, end_date, days, reason) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		lr.ID, lr.EmployeeID, lr.Typ, lr.Status, lr.StartDate, lr.EndDate, lr.Days, lr.Reason)
	if err != nil {
		return nil, err
	}
	return &lr, nil
}

func (s *LeaveService) Approve(ctx context.Context, id uuid.UUID, approver uuid.UUID, approve bool, companyID string) error {
	status := "rejected"
	if approve {
		status = "approved"
	}
	now := time.Now()
	ct, err := s.pg.Exec(ctx, `UPDATE hr_leave_requests SET status=$2, approver_id=$3, decided_at=$4
		WHERE id=$1 AND employee_id IN (SELECT id FROM hr_employees WHERE company_id=$5)`, id, status, approver, now, companyID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("Urlaubsantrag nicht gefunden")
	}
	return nil
}

func (s *LeaveService) Decide(ctx context.Context, id uuid.UUID, approver uuid.UUID, approve bool, companyID string) error {
	return s.Approve(ctx, id, approver, approve, companyID)
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
	rows, err := s.pg.Query(ctx, `SELECT id, employee_id, typ, status, start_date, end_date, days, COALESCE(reason,''), approver_id, decided_at, created_at FROM hr_leave_requests `+where+` ORDER BY created_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LeaveRequest, 0)
	for rows.Next() {
		var lr LeaveRequest
		var appr *uuid.UUID
		var decided *time.Time
		if err := rows.Scan(&lr.ID, &lr.EmployeeID, &lr.Typ, &lr.Status, &lr.StartDate, &lr.EndDate, &lr.Days, &lr.Reason, &appr, &decided, &lr.CreatedAt); err != nil {
			return nil, err
		}
		lr.ApproverID = appr
		lr.DecidedAt = decided
		out = append(out, lr)
	}
	return out, nil
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
