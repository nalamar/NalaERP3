package fleet

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Termine je Fahrzeug (ADR 0032 §3 mit Praezisierung G.2, Backlog
// G.2.2.1): HU, UVV, Wartung, Versicherung, Leasingende, Sonstiges. Ein
// Termin ist PLANUNG - Starttermin, Intervall, Notiz sind aenderbar, er
// wird stillgelegt statt geloescht. Die Belege sind die Erledigungen
// (G.2.2.2); ihr Folgetermin loest den Starttermin ab.

// Name aus Migration 100 (Uebersetzung statt Vorabpruefung).
const scheduleUniqueIndex = "idx_fleet_vehicle_schedules_unique"

type Schedule struct {
	ID              uuid.UUID  `json:"id"`
	VehicleID       uuid.UUID  `json:"vehicle_id"`
	Art             string     `json:"art"`
	Bezeichnung     string     `json:"bezeichnung"`
	Starttermin     *time.Time `json:"starttermin"`
	StartKm         *int       `json:"start_km"`
	IntervallMonate *int       `json:"intervall_monate"`
	IntervallKm     *int       `json:"intervall_km"`
	Notiz           string     `json:"notiz"`
	Aktiv           bool       `json:"aktiv"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ScheduleInput legt einen Termin an. IntervallMonate nil = Vorschlag der
// Art (DefaultIntervalMonths: HU 24, UVV und Versicherung 12); wer keinen
// will, setzt ihn danach mit UpdateSchedule auf nil.
type ScheduleInput struct {
	VehicleID       uuid.UUID  `json:"vehicle_id"`
	Art             string     `json:"art"`
	Bezeichnung     string     `json:"bezeichnung"`
	Starttermin     *time.Time `json:"starttermin"`
	StartKm         *int       `json:"start_km"`
	IntervallMonate *int       `json:"intervall_monate"`
	IntervallKm     *int       `json:"intervall_km"`
	Notiz           string     `json:"notiz"`
}

// ScheduleUpdate ersetzt ALLE aenderbaren Angaben (wie VehicleInput). Art
// und Bezeichnung sind nicht aenderbar - ein anderer Termin ist ein neuer
// Termin; der alte wird stillgelegt, seine Erledigungen bleiben.
type ScheduleUpdate struct {
	Starttermin     *time.Time `json:"starttermin"`
	StartKm         *int       `json:"start_km"`
	IntervallMonate *int       `json:"intervall_monate"`
	IntervallKm     *int       `json:"intervall_km"`
	Notiz           string     `json:"notiz"`
	Aktiv           bool       `json:"aktiv"`
}

// ScheduleWithStanding ist ein Termin mit seinem Stand am Abfragetag.
type ScheduleWithStanding struct {
	Schedule
	Standing ScheduleStanding `json:"standing"`
}

type ScheduleService struct{ pg *pgxpool.Pool }

func NewScheduleService(pg *pgxpool.Pool) *ScheduleService { return &ScheduleService{pg: pg} }

const scheduleColumns = `s.id, s.vehicle_id, s.art, s.bezeichnung, s.starttermin, s.start_km, s.intervall_monate, s.intervall_km, s.notiz, s.aktiv, s.created_at`

func scanSchedule(row pgx.Row) (*Schedule, error) {
	var s Schedule
	if err := row.Scan(&s.ID, &s.VehicleID, &s.Art, &s.Bezeichnung, &s.Starttermin, &s.StartKm, &s.IntervallMonate, &s.IntervallKm, &s.Notiz, &s.Aktiv, &s.CreatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

// normalizeStart: HU zum Monatsende, sonst der Kalendertag.
func normalizeStart(art string, d *time.Time) *time.Time {
	if d == nil {
		return nil
	}
	n := NormalizeDue(art, *d)
	return &n
}

// lockActiveVehicle sperrt ein Fahrzeug des Mandanten und prueft, dass es
// nicht abgemeldet ist. Termine und Erledigungen sperren dieselbe Zeile -
// eine gleichzeitige Abmeldung sieht sie ganz oder gar nicht.
func lockActiveVehicle(ctx context.Context, tx pgx.Tx, vehicleID uuid.UUID, companyID string) (erstzulassung *time.Time, err error) {
	var abgemeldetAm *time.Time
	if err := tx.QueryRow(ctx, `SELECT abgemeldet_am, erstzulassung FROM fleet_vehicles WHERE id=$1 AND company_id=$2 FOR UPDATE`, vehicleID, companyID).
		Scan(&abgemeldetAm, &erstzulassung); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Fahrzeug nicht gefunden")
		}
		return nil, err
	}
	if abgemeldetAm != nil {
		return nil, errors.New("Fahrzeug ist abgemeldet")
	}
	return erstzulassung, nil
}

func (s *ScheduleService) CreateSchedule(ctx context.Context, in ScheduleInput, actorUserID, companyID string) (*Schedule, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.VehicleID == uuid.Nil {
		return nil, errors.New("vehicle_id erforderlich")
	}
	in.Art = strings.TrimSpace(in.Art)
	in.Bezeichnung = strings.TrimSpace(in.Bezeichnung)
	if err := ValidateScheduleInput(in.Art, in.Bezeichnung, in.StartKm, in.IntervallMonate, in.IntervallKm); err != nil {
		return nil, err
	}
	if in.IntervallMonate == nil {
		in.IntervallMonate = DefaultIntervalMonths(in.Art)
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := lockActiveVehicle(ctx, tx, in.VehicleID, companyID); err != nil {
		return nil, err
	}
	sc, err := scanSchedule(tx.QueryRow(ctx, `
		INSERT INTO fleet_vehicle_schedules AS s (id, vehicle_id, art, bezeichnung, starttermin, start_km, intervall_monate, intervall_km, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''))
		RETURNING `+scheduleColumns,
		uuid.New(), in.VehicleID, in.Art, in.Bezeichnung, normalizeStart(in.Art, in.Starttermin), in.StartKm, in.IntervallMonate, in.IntervallKm,
		strings.TrimSpace(in.Notiz), actorUserID))
	if err != nil {
		if isUniqueViolation(err, scheduleUniqueIndex) {
			return nil, errors.New("Termin dieser Art existiert bereits für das Fahrzeug")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return sc, nil
}

// UpdateSchedule ersetzt die Planung eines Termins. Auch ein stillgelegter
// Termin darf wieder aktiviert werden. Am abgemeldeten Fahrzeug wird nichts
// mehr geplant.
func (s *ScheduleService) UpdateSchedule(ctx context.Context, id uuid.UUID, in ScheduleUpdate, companyID string) (*Schedule, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var vehicleID uuid.UUID
	var art string
	if err := tx.QueryRow(ctx, `
		SELECT s.vehicle_id, s.art FROM fleet_vehicle_schedules s
		  JOIN fleet_vehicles v ON v.id = s.vehicle_id
		 WHERE s.id=$1 AND v.company_id=$2`, id, companyID).Scan(&vehicleID, &art); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Termin nicht gefunden")
		}
		return nil, err
	}
	// Art und Bezeichnung sind nicht aenderbar; geprueft werden die Zahlen.
	if err := ValidateScheduleNumbers(in.StartKm, in.IntervallMonate, in.IntervallKm); err != nil {
		return nil, err
	}
	if _, err := lockActiveVehicle(ctx, tx, vehicleID, companyID); err != nil {
		return nil, err
	}
	sc, err := scanSchedule(tx.QueryRow(ctx, `
		UPDATE fleet_vehicle_schedules AS s
		   SET starttermin=$2, start_km=$3, intervall_monate=$4, intervall_km=$5, notiz=$6, aktiv=$7
		 WHERE s.id=$1
		RETURNING `+scheduleColumns,
		id, normalizeStart(art, in.Starttermin), in.StartKm, in.IntervallMonate, in.IntervallKm, strings.TrimSpace(in.Notiz), in.Aktiv))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return sc, nil
}

// loadActiveCompletions laedt die nicht stornierten Erledigungen der
// genannten Termine, gruppiert nach Termin.
func loadActiveCompletions(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, scheduleIDs []uuid.UUID) (map[uuid.UUID][]Completion, error) {
	out := make(map[uuid.UUID][]Completion)
	if len(scheduleIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT `+completionColumns+` FROM fleet_vehicle_completions c WHERE c.schedule_id = ANY($1) AND c.status='active'`, scheduleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanCompletion(rows)
		if err != nil {
			return nil, err
		}
		out[c.ScheduleID] = append(out[c.ScheduleID], *c)
	}
	return out, rows.Err()
}

// loadKmStand liefert je Fahrzeug den zuletzt bekannten Kilometerstand
// (G.3e): den hoechsten aus nicht stornierten Fahrten und nicht
// stornierten Erledigungen mit km-Stand. Fahrzeuge ohne Angabe fehlen.
func loadKmStand(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, vehicleIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	out := make(map[uuid.UUID]int)
	if len(vehicleIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT vehicle_id, max(km) FROM (
		SELECT t.vehicle_id, t.km_ende AS km FROM fleet_trips t WHERE t.vehicle_id = ANY($1) AND t.status='active'
		UNION ALL
		SELECT s.vehicle_id, c.km_stand FROM fleet_vehicle_completions c JOIN fleet_vehicle_schedules s ON s.id = c.schedule_id
		 WHERE s.vehicle_id = ANY($1) AND c.status='active' AND c.km_stand IS NOT NULL
	) k GROUP BY vehicle_id`, vehicleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var km int
		if err := rows.Scan(&id, &km); err != nil {
			return nil, err
		}
		out[id] = km
	}
	return out, rows.Err()
}

// kmOf liefert den km-Stand als Zeiger oder nil, wenn keiner bekannt ist.
func kmOf(m map[uuid.UUID]int, id uuid.UUID) *int {
	km, ok := m[id]
	if !ok {
		return nil
	}
	return &km
}

const completionColumns = `c.id, c.schedule_id, c.erledigt_am, c.ergebnis, c.km_stand, c.naechster_termin, c.naechster_km, c.pruefer, c.notiz,
	c.status, c.storno_reason, c.storniert_at, c.storniert_by_user_id, c.created_by_user_id, c.created_at`

func scanCompletion(row pgx.Row) (*Completion, error) {
	var c Completion
	if err := row.Scan(&c.ID, &c.ScheduleID, &c.ErledigtAm, &c.Ergebnis, &c.KmStand, &c.NaechsterTermin, &c.NaechsterKm, &c.Pruefer, &c.Notiz,
		&c.Status, &c.StornoReason, &c.StorniertAt, &c.StorniertByUserID, &c.CreatedByUserID, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListSchedules liefert die Termine eines Fahrzeugs mit ihrem Stand am Tag
// today, nach Art und Bezeichnung; stillgelegte nur auf Wunsch. Auch fuer
// ein abgemeldetes Fahrzeug lesbar.
func (s *ScheduleService) ListSchedules(ctx context.Context, vehicleID uuid.UUID, includeInactive bool, today time.Time, warnDays int, companyID string) ([]ScheduleWithStanding, error) {
	var owned bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fleet_vehicles WHERE id=$1 AND company_id=$2)`, vehicleID, companyID).Scan(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Fahrzeug nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `SELECT `+scheduleColumns+` FROM fleet_vehicle_schedules s
		WHERE s.vehicle_id=$1 AND (s.aktiv OR $2)
		ORDER BY array_position(ARRAY['hu','uvv','wartung','versicherung','leasing','sonstige'], s.art), lower(s.bezeichnung), s.id`, vehicleID, includeInactive)
	if err != nil {
		return nil, err
	}
	schedules := make([]Schedule, 0)
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		schedules = append(schedules, *sc)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(schedules))
	for i, sc := range schedules {
		ids[i] = sc.ID
	}
	completions, err := loadActiveCompletions(ctx, s.pg, ids)
	if err != nil {
		return nil, err
	}
	kms, err := loadKmStand(ctx, s.pg, []uuid.UUID{vehicleID})
	if err != nil {
		return nil, err
	}
	out := make([]ScheduleWithStanding, 0, len(schedules))
	for _, sc := range schedules {
		st := EvaluateSchedule(sc.Art, sc.Starttermin, sc.StartKm, completions[sc.ID], today, warnDays)
		out = append(out, ScheduleWithStanding{
			Schedule: sc,
			Standing: ApplyKmDue(st, kmOf(kms, vehicleID), DefaultScheduleWarnKm),
		})
	}
	return out, nil
}

// GetSchedule liefert einen Termin des Mandanten.
func (s *ScheduleService) GetSchedule(ctx context.Context, id uuid.UUID, companyID string) (*Schedule, error) {
	sc, err := scanSchedule(s.pg.QueryRow(ctx, `SELECT `+scheduleColumns+` FROM fleet_vehicle_schedules s
		JOIN fleet_vehicles v ON v.id = s.vehicle_id WHERE s.id=$1 AND v.company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Termin nicht gefunden")
		}
		return nil, err
	}
	return sc, nil
}
