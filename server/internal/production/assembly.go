package production

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Montagetermine am Fertigungsauftrag (ADR 0038 Frage 6 (a), Backlog
// H.3.6b): anlegen, aendern, stornieren, Liste je Auftrag. Das Projekt
// kommt vom Auftrag; Team und Fahrzeug muessen zum Mandanten gehoeren.
// Jede Aenderung im entity_change_log des Auftrags. Hinweise in
// assembly_hints.go (H.3.6c).

var (
	ErrAppointmentNotFound    = errors.New("Montagetermin nicht gefunden")
	errAppointmentCancelled   = errors.New("Montagetermin ist storniert")
	errAppointmentStorniert   = errors.New("Montagetermin ist bereits storniert")
	errAssemblyTeamNotFound   = errors.New("Team nicht gefunden")
	errAssemblyVehicleMissing = errors.New("Fahrzeug nicht gefunden")
	errAssemblyVehicleOff     = errors.New("Fahrzeug ist abgemeldet")
)

type Appointment struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"project_id"`
	OrderID       *uuid.UUID `json:"order_id"`
	OrderNummer   string     `json:"order_nummer"`
	ProjectNummer string     `json:"project_nummer"`
	ProjectName   string     `json:"project_name"`
	Von           time.Time  `json:"von"`
	Bis           time.Time  `json:"bis"`
	TeamID        *uuid.UUID `json:"team_id"`
	TeamName      string     `json:"team_name"`
	VehicleID     *uuid.UUID `json:"vehicle_id"`
	Kennzeichen   string     `json:"kennzeichen"`
	Baustelle     string     `json:"baustelle"`
	Notiz         string     `json:"notiz"`
	StornoReason  string     `json:"storno_reason"`
	StorniertAt   *time.Time `json:"storniert_at"`
	CreatedAt     time.Time  `json:"created_at"`
	Hinweise      []string   `json:"hinweise"`
}

const appointmentSelect = `SELECT a.id, a.project_id, a.order_id, COALESCE(o.nummer, ''), p.nummer, p.name, a.von, a.bis, a.team_id, COALESCE(t.name, ''),
	a.vehicle_id, COALESCE(v.kennzeichen, ''), a.baustelle, a.notiz, a.storno_reason, a.storniert_at, a.created_at
	  FROM production_assembly_appointments a
	  JOIN projects p ON p.id = a.project_id
	  LEFT JOIN production_orders o ON o.id = a.order_id
	  LEFT JOIN hr_teams t ON t.id = a.team_id
	  LEFT JOIN fleet_vehicles v ON v.id = a.vehicle_id`

func scanAppointment(row pgx.Row) (*Appointment, error) {
	var x Appointment
	if err := row.Scan(&x.ID, &x.ProjectID, &x.OrderID, &x.OrderNummer, &x.ProjectNummer, &x.ProjectName, &x.Von, &x.Bis, &x.TeamID, &x.TeamName,
		&x.VehicleID, &x.Kennzeichen, &x.Baustelle, &x.Notiz, &x.StornoReason, &x.StorniertAt, &x.CreatedAt); err != nil {
		return nil, err
	}
	x.Hinweise = []string{}
	return &x, nil
}

func getAppointment(ctx context.Context, q querier, orderID, id uuid.UUID, companyID string) (*Appointment, error) {
	x, err := scanAppointment(q.QueryRow(ctx, appointmentSelect+`
		 WHERE a.id=$1 AND a.order_id=$2 AND a.company_id=$3`, id, orderID, companyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAppointmentNotFound
	}
	return x, err
}

// ListAppointments: alle Termine des Auftrags mit Stornos, nach Beginn.
func (s *Service) ListAppointments(ctx context.Context, orderID uuid.UUID, companyID string) ([]Appointment, error) {
	var exists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM production_orders WHERE id=$1 AND company_id=$2)`,
		orderID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return s.queryAppointments(ctx, `
		 WHERE a.order_id=$1 AND a.company_id=$2 ORDER BY a.von, a.created_at, a.id`, companyID, orderID, companyID)
}

// queryAppointments liest die Termine zu where (mit args, Mandant
// eingeschlossen) und setzt ihre Hinweise zum heutigen Tag.
func (s *Service) queryAppointments(ctx context.Context, where, companyID string, args ...any) ([]Appointment, error) {
	rows, err := s.pg.Query(ctx, appointmentSelect+where, args...)
	if err != nil {
		return nil, err
	}
	out := []Appointment{}
	for rows.Next() {
		x, err := scanAppointment(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, fillAssemblyHints(ctx, s.pg, out, companyID, time.Now())
}

// withHints liefert den Termin nach dem Commit mit seinen Hinweisen.
func (s *Service) withHints(ctx context.Context, x *Appointment, companyID string) (*Appointment, error) {
	list := []Appointment{*x}
	if err := fillAssemblyHints(ctx, s.pg, list, companyID, time.Now()); err != nil {
		return nil, err
	}
	return &list[0], nil
}

// lockOrderForAssembly sperrt den Auftrag des Mandanten und liefert Status
// und Projekt.
func lockOrderForAssembly(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, companyID string) (status string, project uuid.UUID, err error) {
	err = tx.QueryRow(ctx, `SELECT status, project_id FROM production_orders WHERE id=$1 AND company_id=$2 FOR UPDATE`,
		orderID, companyID).Scan(&status, &project)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return status, project, err
}

// checkAssemblyRefs: Team und Fahrzeug des Mandanten; ein Fahrzeug, das
// bis zum Ende der Montage abgemeldet ist, wird abgelehnt.
func checkAssemblyRefs(ctx context.Context, tx pgx.Tx, in AppointmentInput, companyID string) error {
	if in.TeamID != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM hr_teams WHERE id=$1 AND company_id=$2)`,
			*in.TeamID, companyID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return errAssemblyTeamNotFound
		}
	}
	if in.VehicleID != nil {
		var abgemeldet *time.Time
		err := tx.QueryRow(ctx, `SELECT abgemeldet_am FROM fleet_vehicles WHERE id=$1 AND company_id=$2`,
			*in.VehicleID, companyID).Scan(&abgemeldet)
		if errors.Is(err, pgx.ErrNoRows) {
			return errAssemblyVehicleMissing
		}
		if err != nil {
			return err
		}
		if abgemeldet != nil && !dateOnly(*abgemeldet).After(*in.Bis) {
			return errAssemblyVehicleOff
		}
	}
	return nil
}

// prepareAssembly normalisiert und prueft die Eingabe fuer den Auftrag und
// sperrt ihn; Status und Bezuege muessen passen.
func prepareAssembly(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, in AppointmentInput, companyID string) (AppointmentInput, uuid.UUID, error) {
	in.OrderID = &orderID
	in = in.Normalize()
	if err := in.Validate(); err != nil {
		return in, uuid.Nil, err
	}
	status, project, err := lockOrderForAssembly(ctx, tx, orderID, companyID)
	if err != nil {
		return in, uuid.Nil, err
	}
	if err := AssemblyAllowed(status); err != nil {
		return in, uuid.Nil, err
	}
	return in, project, checkAssemblyRefs(ctx, tx, in, companyID)
}

// CreateAppointment legt einen Montagetermin zum Auftrag an.
func (s *Service) CreateAppointment(ctx context.Context, orderID uuid.UUID, in AppointmentInput, actorUserID, companyID string) (*Appointment, error) {
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	in, project, err := prepareAssembly(ctx, tx, orderID, in, companyID)
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO production_assembly_appointments
		(id, company_id, project_id, order_id, von, bis, team_id, vehicle_id, baustelle, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id, companyID, project, orderID, *in.Von, *in.Bis,
		in.TeamID, in.VehicleID, in.Baustelle, in.Notiz, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	after, err := getAppointment(ctx, tx, orderID, id, companyID)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "assembly_created", actorUserID, nil, after); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.withHints(ctx, after, companyID)
}

// UpdateAppointment aendert Zeitraum, Team, Fahrzeug, Baustelle und Notiz
// eines nicht stornierten Termins.
func (s *Service) UpdateAppointment(ctx context.Context, orderID, id uuid.UUID, in AppointmentInput, actorUserID, companyID string) (*Appointment, error) {
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	in, _, err = prepareAssembly(ctx, tx, orderID, in, companyID)
	if err != nil {
		return nil, err
	}
	before, err := getAppointment(ctx, tx, orderID, id, companyID)
	if err != nil {
		return nil, err
	}
	if before.StorniertAt != nil {
		return nil, errAppointmentCancelled
	}
	if _, err := tx.Exec(ctx, `UPDATE production_assembly_appointments
		SET von=$2, bis=$3, team_id=$4, vehicle_id=$5, baustelle=$6, notiz=$7 WHERE id=$1`,
		id, *in.Von, *in.Bis, in.TeamID, in.VehicleID, in.Baustelle, in.Notiz); err != nil {
		return nil, err
	}
	after, err := getAppointment(ctx, tx, orderID, id, companyID)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "assembly_updated", actorUserID, before, after); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.withHints(ctx, after, companyID)
}

// CancelAppointment storniert einen Termin mit Begruendung - auch bei
// stornierten Auftraegen.
func (s *Service) CancelAppointment(ctx context.Context, orderID, id uuid.UUID, reason, actorUserID, companyID string) (*Appointment, error) {
	reason, err := ValidateAssemblyCancelReason(reason)
	if err != nil {
		return nil, err
	}
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, _, err := lockOrderForAssembly(ctx, tx, orderID, companyID); err != nil {
		return nil, err
	}
	before, err := getAppointment(ctx, tx, orderID, id, companyID)
	if err != nil {
		return nil, err
	}
	if before.StorniertAt != nil {
		return nil, errAppointmentStorniert
	}
	if _, err := tx.Exec(ctx, `UPDATE production_assembly_appointments
		SET storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3 WHERE id=$1`,
		id, reason, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	after, err := getAppointment(ctx, tx, orderID, id, companyID)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "assembly_cancelled", actorUserID, before, after); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return after, nil
}
