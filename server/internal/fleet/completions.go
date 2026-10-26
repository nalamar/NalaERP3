package fleet

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/hr"
)

// Erledigungen der Termine (ADR 0032 §3 mit Praezisierung G.2, Backlog
// G.2.2.2): HU, UVV-Pruefung, Wartung, Fristen. Unveraenderlich wie die
// Pruefungen der Betriebsmittel - die UVV-Pruefung ist bis zur naechsten
// schriftlich aufzubewahren (DGUV Vorschrift 70). Fehler werden storniert
// und neu erfasst.

type CompletionInput struct {
	ScheduleID uuid.UUID `json:"schedule_id"`
	ErledigtAm time.Time `json:"erledigt_am"`
	Ergebnis   string    `json:"ergebnis"`
	KmStand    *int      `json:"km_stand"`
	// NaechsterTermin: nil = aus dem Intervall des Termins vorschlagen
	// (NextDue). Angegeben wird er bei der HU auf das Monatsende gelegt.
	NaechsterTermin *time.Time `json:"naechster_termin"`
	// NaechsterKm: nil = km-Stand + Intervall-km des Termins, wenn beides
	// bekannt ist.
	NaechsterKm *int   `json:"naechster_km"`
	Pruefer     string `json:"pruefer"`
	Notiz       string `json:"notiz"`
}

func nullableUserID(userID string) *string {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	return &userID
}

// RecordCompletion erfasst eine Erledigung. Der Folgetermin wird HIER
// festgelegt und gespeichert: aendert der Betrieb spaeter das Intervall,
// gilt eine erfolgte Erledigung nicht rueckwirkend anders.
//
// Die Fahrzeug-Zeile wird gesperrt (lockActiveVehicle) - eine gleichzeitige
// Abmeldung sieht die Erledigung ganz oder gar nicht.
func (s *ScheduleService) RecordCompletion(ctx context.Context, in CompletionInput, actorUserID, companyID string) (*Completion, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.ScheduleID == uuid.Nil {
		return nil, errors.New("schedule_id erforderlich")
	}
	in.Ergebnis = strings.TrimSpace(in.Ergebnis)
	in.Pruefer = strings.TrimSpace(in.Pruefer)
	in.Notiz = strings.TrimSpace(in.Notiz)

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var vehicleID uuid.UUID
	var art string
	var aktiv bool
	var intervallMonate, intervallKm *int
	if err := tx.QueryRow(ctx, `
		SELECT s.vehicle_id, s.art, s.aktiv, s.intervall_monate, s.intervall_km
		  FROM fleet_vehicle_schedules s
		  JOIN fleet_vehicles v ON v.id = s.vehicle_id
		 WHERE s.id=$1 AND v.company_id=$2`, in.ScheduleID, companyID).Scan(&vehicleID, &art, &aktiv, &intervallMonate, &intervallKm); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Termin nicht gefunden")
		}
		return nil, err
	}
	erstzulassung, err := lockActiveVehicle(ctx, tx, vehicleID, companyID)
	if err != nil {
		return nil, err
	}
	if !aktiv {
		return nil, errors.New("Termin ist stillgelegt")
	}

	naechster := in.NaechsterTermin
	if naechster != nil {
		n := NormalizeDue(art, *naechster)
		naechster = &n
	} else if !in.ErledigtAm.IsZero() {
		naechster = NextDue(art, in.ErledigtAm, intervallMonate)
	}
	naechsterKm := in.NaechsterKm
	if naechsterKm == nil && in.KmStand != nil && intervallKm != nil {
		k := *in.KmStand + *intervallKm
		naechsterKm = &k
	}
	if err := ValidateCompletion(art, in.ErledigtAm, in.Ergebnis, naechster, in.KmStand, naechsterKm, erstzulassung, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}

	c, err := scanCompletion(tx.QueryRow(ctx, `
		INSERT INTO fleet_vehicle_completions AS c (id, schedule_id, erledigt_am, ergebnis, km_stand, naechster_termin, naechster_km, pruefer, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+completionColumns,
		uuid.New(), in.ScheduleID, dateOnly(in.ErledigtAm), in.Ergebnis, in.KmStand, naechster, naechsterKm, in.Pruefer, in.Notiz, nullableUserID(actorUserID)))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// StornoCompletion storniert eine falsch erfasste Erledigung. Einmalig,
// mit Begruendung. Ist sie die juengste, gilt danach wieder die vorherige
// (oder der Starttermin) - auch eine Sperre durch eine HU mit Maengeln
// faellt so weg. Auch am abgemeldeten Fahrzeug moeglich: eine Berichtigung
// der Historie bleibt erlaubt.
func (s *ScheduleService) StornoCompletion(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*Completion, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Erledigungs-ID erforderlich")
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
		SELECT c.status
		  FROM fleet_vehicle_completions c
		  JOIN fleet_vehicle_schedules s ON s.id = c.schedule_id
		  JOIN fleet_vehicles v ON v.id = s.vehicle_id
		 WHERE c.id=$1 AND v.company_id=$2
		   FOR UPDATE OF c`, id, companyID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Erledigung nicht gefunden")
		}
		return nil, err
	}
	if status == CompletionStorniert {
		return nil, errors.New("Erledigung ist bereits storniert")
	}
	c, err := scanCompletion(tx.QueryRow(ctx, `
		UPDATE fleet_vehicle_completions AS c SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3
		 WHERE c.id=$1 AND c.status='active'
		RETURNING `+completionColumns, id, reason, nullableUserID(actorUserID)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Erledigung ist bereits storniert")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// ListCompletions liefert die Erledigungen eines Termins, die juengste
// zuerst (dieselbe Reihenfolge, nach der EvaluateSchedule die massgebliche
// waehlt). Stornierte nur auf Wunsch.
func (s *ScheduleService) ListCompletions(ctx context.Context, scheduleID uuid.UUID, includeStorniert bool, companyID string) ([]Completion, error) {
	if _, err := s.GetSchedule(ctx, scheduleID, companyID); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `SELECT `+completionColumns+` FROM fleet_vehicle_completions c
		WHERE c.schedule_id=$1 AND (c.status='active' OR $2)
		ORDER BY c.erledigt_am DESC, c.created_at DESC, c.id`, scheduleID, includeStorniert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Completion, 0)
	for rows.Next() {
		c, err := scanCompletion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}
