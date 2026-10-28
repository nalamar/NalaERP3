package fleet

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/hr"
)

// Fahrtenbuch (ADR 0032 §6, Backlog G.3c.2): Fahrten erfassen, stornieren,
// lesen. Geschlossene Form - eine Fahrt wird nie geaendert und nie
// geloescht; erlaubt ist allein das einmalige Storno. Die Regeln selbst
// stehen in trip_rules.go.

// Namen der partiellen UNIQUE-Indizes aus Migration 103.
const (
	tripKmBeginnIndex = "idx_fleet_trips_km_beginn"
	tripKmEndeIndex   = "idx_fleet_trips_km_ende"
)

type Trip struct {
	ID                uuid.UUID  `json:"id"`
	VehicleID         uuid.UUID  `json:"vehicle_id"`
	Kennzeichen       string     `json:"kennzeichen"`
	Fahrzeug          string     `json:"fahrzeug"`
	EmployeeID        uuid.UUID  `json:"employee_id"`
	MitarbeiterName   string     `json:"mitarbeiter_name"`
	Datum             time.Time  `json:"datum"`
	KmBeginn          int        `json:"km_beginn"`
	KmEnde            int        `json:"km_ende"`
	Km                int        `json:"km"`
	Fahrtart          string     `json:"fahrtart"`
	Ziel              string     `json:"ziel"`
	Zweck             string     `json:"zweck"`
	Geschaeftspartner string     `json:"geschaeftspartner"`
	Notiz             string     `json:"notiz"`
	Status            string     `json:"status"`
	StornoReason      string     `json:"storno_reason,omitempty"`
	StorniertAt       *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID *string    `json:"storniert_by_user_id,omitempty"`
	CreatedByUserID   *string    `json:"created_by_user_id"`
	// Erfassungszeitpunkt: zeigt, ob zeitnah eingetragen wurde.
	CreatedAt time.Time `json:"created_at"`
}

// TripFilter schraenkt die Liste ein. Ohne Angaben: alle nicht stornierten
// Fahrten des Mandanten.
type TripFilter struct {
	VehicleID        *uuid.UUID
	EmployeeID       *uuid.UUID
	Von              *time.Time
	Bis              *time.Time
	IncludeStorniert bool
}

// TripChainState ist der Stand der Kilometerkette eines Fahrzeugs.
type TripChainState struct {
	// KmStand: Ende der letzten nicht stornierten Fahrt, nil ohne Fahrt.
	KmStand *int      `json:"km_stand"`
	Luecken []TripGap `json:"luecken"`
}

type TripService struct {
	pg *pgxpool.Pool
}

func NewTripService(pg *pgxpool.Pool) *TripService {
	return &TripService{pg: pg}
}

const tripColumns = `t.id, t.vehicle_id, v.kennzeichen, v.bezeichnung, t.employee_id,
	btrim(e.first_name || ' ' || e.last_name), t.datum, t.km_beginn, t.km_ende, t.fahrtart, t.ziel, t.zweck,
	t.geschaeftspartner, t.notiz, t.status, t.storno_reason, t.storniert_at, t.storniert_by_user_id,
	t.created_by_user_id, t.created_at`

const tripFrom = `
	  FROM fleet_trips t
	  JOIN fleet_vehicles v ON v.id = t.vehicle_id
	  JOIN hr_employees e ON e.id = t.employee_id`

func scanTrip(row pgx.Row) (*Trip, error) {
	var x Trip
	if err := row.Scan(&x.ID, &x.VehicleID, &x.Kennzeichen, &x.Fahrzeug, &x.EmployeeID,
		&x.MitarbeiterName, &x.Datum, &x.KmBeginn, &x.KmEnde, &x.Fahrtart, &x.Ziel, &x.Zweck,
		&x.Geschaeftspartner, &x.Notiz, &x.Status, &x.StornoReason, &x.StorniertAt, &x.StorniertByUserID,
		&x.CreatedByUserID, &x.CreatedAt); err != nil {
		return nil, err
	}
	x.Km = x.KmEnde - x.KmBeginn
	return &x, nil
}

// activeTripSpans liefert die nicht stornierten Fahrten eines Fahrzeugs
// fuer die Kette.
func activeTripSpans(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, vehicleID uuid.UUID) ([]TripSpan, error) {
	rows, err := q.Query(ctx, `SELECT datum, km_beginn, km_ende FROM fleet_trips WHERE vehicle_id=$1 AND status='active'`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TripSpan, 0)
	for rows.Next() {
		var sp TripSpan
		if err := rows.Scan(&sp.Datum, &sp.KmBeginn, &sp.KmEnde); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// RecordTrip erfasst eine Fahrt.
//
// Die Fahrzeugzeile wird gesperrt - dieselbe Sperre wie beim Abmelden.
// Zwei gleichzeitige Fahrten desselben Fahrzeugs sehen einander also, und
// die Kette wird gegen den vollstaendigen Stand geprueft; die Indizes aus
// Migration 103 sichern zusaetzlich ab. Ein abgemeldetes Fahrzeug nimmt
// noch nachgetragene Fahrten bis zum Tag der Abmeldung an.
func (s *TripService) RecordTrip(ctx context.Context, in TripInput, actorUserID, companyID string) (*Trip, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.VehicleID == uuid.Nil {
		return nil, errors.New("vehicle_id erforderlich")
	}
	if in.EmployeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	in = NormalizeTripInput(in)
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var abgemeldetAm, erstzulassung *time.Time
	var fahrtenbuch bool
	if err := tx.QueryRow(ctx, `SELECT abgemeldet_am, erstzulassung, fahrtenbuch FROM fleet_vehicles WHERE id=$1 AND company_id=$2 FOR UPDATE`, in.VehicleID, companyID).
		Scan(&abgemeldetAm, &erstzulassung, &fahrtenbuch); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Fahrzeug nicht gefunden")
		}
		return nil, err
	}
	if !fahrtenbuch {
		return nil, errors.New("Für dieses Fahrzeug wird kein Fahrtenbuch geführt")
	}
	if err := ValidateTripInput(in, erstzulassung, abgemeldetAm, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}

	var active bool
	if err := tx.QueryRow(ctx, `SELECT active FROM hr_employees WHERE id=$1 AND company_id=$2`, in.EmployeeID, companyID).Scan(&active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Mitarbeiter nicht gefunden")
		}
		return nil, err
	}
	if !active {
		return nil, errors.New("Mitarbeiter ist nicht aktiv")
	}

	spans, err := activeTripSpans(ctx, tx, in.VehicleID)
	if err != nil {
		return nil, err
	}
	datum := dateOnly(in.Datum)
	if err := CheckTripChain(spans, TripSpan{Datum: datum, KmBeginn: in.KmBeginn, KmEnde: in.KmEnde}); err != nil {
		return nil, err
	}

	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO fleet_trips (id, vehicle_id, employee_id, datum, km_beginn, km_ende, fahrtart, ziel, zweck, geschaeftspartner, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		id, in.VehicleID, in.EmployeeID, datum, in.KmBeginn, in.KmEnde, in.Fahrtart, in.Ziel, in.Zweck,
		in.Geschaeftspartner, in.Notiz, nullableUserID(actorUserID)); err != nil {
		if isUniqueViolation(err, tripKmBeginnIndex) || isUniqueViolation(err, tripKmEndeIndex) {
			return nil, errors.New("Kilometerbereich überschneidet sich mit einer anderen Fahrt")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetTrip(ctx, id, companyID)
}

func (s *TripService) GetTrip(ctx context.Context, id uuid.UUID, companyID string) (*Trip, error) {
	x, err := scanTrip(s.pg.QueryRow(ctx, `SELECT `+tripColumns+tripFrom+`
		 WHERE t.id=$1 AND v.company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Fahrt nicht gefunden")
		}
		return nil, err
	}
	return x, nil
}

// ListTrips liefert Fahrten des Mandanten, die juengsten zuerst (Datum,
// dann Kilometerstand). Von/Bis grenzen das Datum ein (jeweils
// einschliesslich); stornierte nur auf Wunsch.
func (s *TripService) ListTrips(ctx context.Context, companyID string, f TripFilter) ([]Trip, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+tripColumns+tripFrom+`
		 WHERE v.company_id=$1
		   AND ($2::uuid IS NULL OR t.vehicle_id=$2)
		   AND ($3::uuid IS NULL OR t.employee_id=$3)
		   AND ($4::date IS NULL OR t.datum >= $4)
		   AND ($5::date IS NULL OR t.datum <= $5)
		   AND (t.status='active' OR $6)
		 ORDER BY t.datum DESC, t.km_beginn DESC, t.id`,
		companyID, f.VehicleID, f.EmployeeID, f.Von, f.Bis, f.IncludeStorniert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Trip, 0)
	for rows.Next() {
		x, err := scanTrip(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

// ChainState liefert den Kilometerstand und die Luecken der Kette eines
// Fahrzeugs des Mandanten.
func (s *TripService) ChainState(ctx context.Context, vehicleID uuid.UUID, companyID string) (*TripChainState, error) {
	var exists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fleet_vehicles WHERE id=$1 AND company_id=$2)`, vehicleID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Fahrzeug nicht gefunden")
	}
	spans, err := activeTripSpans(ctx, s.pg, vehicleID)
	if err != nil {
		return nil, err
	}
	st := &TripChainState{Luecken: TripGaps(spans)}
	for _, sp := range spans {
		if st.KmStand == nil || sp.KmEnde > *st.KmStand {
			km := sp.KmEnde
			st.KmStand = &km
		}
	}
	return st, nil
}

// StornoTrip storniert eine falsch erfasste Fahrt - einmalig, mit
// Begruendung, auch am abgemeldeten Fahrzeug. Die Fahrt bleibt sichtbar;
// ihr Kilometerbereich wird zur Luecke, die eine neue Fahrt fuellen kann.
func (s *TripService) StornoTrip(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*Trip, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Fahrt-ID erforderlich")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("Storno-Begründung erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx, `
		SELECT t.status FROM fleet_trips t
		  JOIN fleet_vehicles v ON v.id = t.vehicle_id
		 WHERE t.id=$1 AND v.company_id=$2
		   FOR UPDATE OF t`, id, companyID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Fahrt nicht gefunden")
		}
		return nil, err
	}
	if status == TripStorniert {
		return nil, errors.New("Fahrt ist bereits storniert")
	}
	ct, err := tx.Exec(ctx, `
		UPDATE fleet_trips SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3
		 WHERE id=$1 AND status='active'
	`, id, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Fahrt ist bereits storniert")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetTrip(ctx, id, companyID)
}
