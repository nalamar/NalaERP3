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

func (s *EmployeeService) ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, name, lead_employee_id FROM hr_teams ORDER BY name`)
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

func (s *EmployeeService) List(ctx context.Context, limit, offset int) ([]Employee, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pg.Query(ctx, `SELECT id, personalnummer, first_name, last_name, COALESCE(email,''), COALESCE(phone,''), hire_date, termination_date, COALESCE(role,''), team_id, COALESCE(location,''), COALESCE(cost_center,''), active FROM hr_employees ORDER BY last_name, first_name LIMIT $1 OFFSET $2`, limit, offset)
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

func (s *EmployeeService) Create(ctx context.Context, e Employee) (*Employee, error) {
	if e.FirstName == "" || e.LastName == "" {
		return nil, errors.New("Vor- und Nachname erforderlich")
	}
	id := uuid.New()
	_, err := s.pg.Exec(ctx, `INSERT INTO hr_employees (id, personalnummer, first_name, last_name, email, phone, hire_date, termination_date, role, team_id, location, cost_center, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		id, e.PersonalNr, e.FirstName, e.LastName, e.Email, e.Phone, e.HireDate, e.Termination, e.Role, e.TeamID, e.Location, e.CostCenter, e.Active)
	if err != nil {
		return nil, err
	}
	e.ID = id
	if e.Active == false {
		e.Active = false
	}
	return &e, nil
}

func (s *EmployeeService) Get(ctx context.Context, id uuid.UUID) (*Employee, error) {
	var e Employee
	var hire, term *time.Time
	var teamID *uuid.UUID
	var pn *string
	err := s.pg.QueryRow(ctx, `SELECT id, personalnummer, first_name, last_name, COALESCE(email,''), COALESCE(phone,''), hire_date, termination_date, COALESCE(role,''), team_id, COALESCE(location,''), COALESCE(cost_center,''), active FROM hr_employees WHERE id=$1`, id).
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

func (s *EmployeeService) Update(ctx context.Context, id uuid.UUID, patch map[string]any) error {
	if len(patch) == 0 {
		return nil
	}
	sets := []string{}
	args := []any{}
	i := 1
	for k, v := range patch {
		switch k {
		case "first_name", "last_name", "email", "phone", "role", "location", "cost_center":
			sets = append(sets, fmt.Sprintf("%s=$%d", k, i))
			args = append(args, v)
			i++
		case "active":
			sets = append(sets, fmt.Sprintf("%s=$%d", k, i))
			args = append(args, v)
			i++
		case "team_id":
			sets = append(sets, fmt.Sprintf("%s=$%d", k, i))
			args = append(args, v)
			i++
		}
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := s.pg.Exec(ctx, `UPDATE hr_employees SET `+strings.Join(sets, ", ")+` WHERE id=$`+fmt.Sprintf("%d", len(args)), args...)
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

func (s *LeaveService) Create(ctx context.Context, lr LeaveRequest) (*LeaveRequest, error) {
	if lr.EmployeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	if lr.Typ == "" {
		lr.Typ = "vacation"
	}
	if lr.StartDate.IsZero() || lr.EndDate.IsZero() {
		return nil, errors.New("start/end erforderlich")
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

func (s *LeaveService) Approve(ctx context.Context, id uuid.UUID, approver uuid.UUID, approve bool) error {
	status := "rejected"
	if approve {
		status = "approved"
	}
	now := time.Now()
	_, err := s.pg.Exec(ctx, `UPDATE hr_leave_requests SET status=$2, approver_id=$3, decided_at=$4 WHERE id=$1`, id, status, approver, now)
	return err
}

func (s *LeaveService) Decide(ctx context.Context, id uuid.UUID, approver uuid.UUID, approve bool) error {
	return s.Approve(ctx, id, approver, approve)
}

func (s *LeaveService) List(ctx context.Context, employeeID *uuid.UUID, limit, offset int) ([]LeaveRequest, error) {
	if limit <= 0 {
		limit = 50
	}
	args := []any{}
	where := ""
	if employeeID != nil {
		where = "WHERE employee_id=$1"
		args = append(args, *employeeID)
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
