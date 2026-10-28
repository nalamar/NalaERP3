package fleet

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/hr"
)

// Fahrerzuordnung (ADR 0032 §4, Backlog G.3a): ein Fahrzeug wird einem
// Mitarbeiter zugeordnet - Muster der Ausgabe von Betriebsmitteln
// (assets.Issue). Eine Zuordnung wird nicht geaendert und nicht geloescht;
// erlaubt sind das einmalige Ende und das Storno.
//
// Eine HU oder UVV mit Maengeln oder nicht bestanden sperrt die Zuordnung.
// Ein ueberfaelliger Termin und eine nicht gueltige Fuehrerscheinkontrolle
// des Fahrers sind Hinweise, keine Sperre (Entscheidung des Nutzers vom
// 2026-10-25).

const (
	AssignmentActive    = "active"
	AssignmentStorniert = "storniert"
)

// Name des partiellen UNIQUE-Index aus Migration 102: hoechstens eine
// offene Zuordnung je Fahrzeug.
const oneOpenAssignmentIndex = "idx_fleet_vehicle_assignments_one_open"

// LicenseCheckCode ist der Code der Art aus dem Vorschlagskatalog (G.3b).
// Wer die Art unter einem anderen Code fuehrt, bekommt immer den Hinweis.
const LicenseCheckCode = "FUEHRERSCHEINKONTROLLE"

type Assignment struct {
	ID                 uuid.UUID  `json:"id"`
	VehicleID          uuid.UUID  `json:"vehicle_id"`
	Kennzeichen        string     `json:"kennzeichen"`
	Fahrzeug           string     `json:"fahrzeug"`
	EmployeeID         uuid.UUID  `json:"employee_id"`
	MitarbeiterName    string     `json:"mitarbeiter_name"`
	Beginn             time.Time  `json:"beginn"`
	Ende               *time.Time `json:"ende"`
	Notiz              string     `json:"notiz"`
	Status             string     `json:"status"`
	StornoReason       string     `json:"storno_reason,omitempty"`
	StorniertAt        *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID  *string    `json:"storniert_by_user_id,omitempty"`
	ZugeordnetByUserID *string    `json:"zugeordnet_by_user_id"`
	BeendetByUserID    *string    `json:"beendet_by_user_id"`
	CreatedAt          time.Time  `json:"created_at"`
	// Hinweise nur in der Antwort auf eine Zuordnung. Sie verhindern sie
	// nicht.
	Hinweise []string `json:"hinweise,omitempty"`
}

type AssignInput struct {
	VehicleID  uuid.UUID `json:"vehicle_id"`
	EmployeeID uuid.UUID `json:"employee_id"`
	Beginn     time.Time `json:"beginn"`
	Notiz      string    `json:"notiz"`
}

// AssignmentFilter schraenkt die Liste ein. Ohne Angaben: alle nicht
// stornierten Zuordnungen des Mandanten.
type AssignmentFilter struct {
	VehicleID        *uuid.UUID
	EmployeeID       *uuid.UUID
	NurOffen         bool
	IncludeStorniert bool
}

type AssignmentService struct {
	pg    *pgxpool.Pool
	quals *hr.QualificationService
}

func NewAssignmentService(pg *pgxpool.Pool) *AssignmentService {
	return &AssignmentService{pg: pg, quals: hr.NewQualificationService(pg)}
}

const assignmentColumns = `x.id, x.vehicle_id, v.kennzeichen, v.bezeichnung, x.employee_id,
	btrim(e.first_name || ' ' || e.last_name), x.beginn, x.ende, x.notiz, x.status, x.storno_reason,
	x.storniert_at, x.storniert_by_user_id, x.zugeordnet_by_user_id, x.beendet_by_user_id, x.created_at`

const assignmentFrom = `
	  FROM fleet_vehicle_assignments x
	  JOIN fleet_vehicles v ON v.id = x.vehicle_id
	  JOIN hr_employees e ON e.id = x.employee_id`

func scanAssignment(row pgx.Row) (*Assignment, error) {
	var x Assignment
	if err := row.Scan(&x.ID, &x.VehicleID, &x.Kennzeichen, &x.Fahrzeug, &x.EmployeeID,
		&x.MitarbeiterName, &x.Beginn, &x.Ende, &x.Notiz, &x.Status, &x.StornoReason,
		&x.StorniertAt, &x.StorniertByUserID, &x.ZugeordnetByUserID, &x.BeendetByUserID, &x.CreatedAt); err != nil {
		return nil, err
	}
	return &x, nil
}

// ValidateAssignmentStart prueft den Beginn ohne Datenbank. Erfasst wird,
// was zugeordnet IST, nicht was geplant ist.
func ValidateAssignmentStart(beginn time.Time, erstzulassung *time.Time, today time.Time) error {
	if beginn.IsZero() {
		return errors.New("Beginn erforderlich")
	}
	if dateOnly(beginn).After(dateOnly(today)) {
		return errors.New("Beginn darf nicht in der Zukunft liegen")
	}
	if erstzulassung != nil && dateOnly(beginn).Before(dateOnly(*erstzulassung)) {
		return errors.New("Beginn darf nicht vor der Erstzulassung liegen")
	}
	return nil
}

// assignmentBlock liefert die Sperre: die erste gesperrte HU/UVV in der
// Reihenfolge der Liste (HU vor UVV).
func assignmentBlock(list []ScheduleWithStanding) error {
	for _, s := range list {
		if s.Standing.Status != TerminstandGesperrt {
			continue
		}
		grund := "nicht bestanden"
		if s.Standing.LetztesErgebnis == ErgebnisMaengel {
			grund = "mit Mängeln"
		}
		return fmt.Errorf("Fahrzeug ist gesperrt: %s %s", TerminLabel(s.Art, s.Bezeichnung), grund)
	}
	return nil
}

// scheduleHints nennt die ueberfaelligen Termine.
func scheduleHints(list []ScheduleWithStanding) []string {
	hints := make([]string, 0)
	for _, s := range list {
		if s.Standing.Status == TerminstandUeberfaellig && s.Standing.FaelligAm != nil {
			hints = append(hints, fmt.Sprintf("%s überfällig seit %s", TerminLabel(s.Art, s.Bezeichnung), s.Standing.FaelligAm.Format("02.01.2006")))
		}
	}
	return hints
}

// licenseHint liefert "" bei gueltiger Fuehrerscheinkontrolle (gueltig
// oder laeuft ab), sonst den Hinweis. Ohne Namen (DSGVO, Logs).
func licenseHint(standings []hr.EmployeeQualificationStanding) string {
	for _, s := range standings {
		if s.Code != LicenseCheckCode {
			continue
		}
		switch s.Status {
		case hr.QualificationStandGueltig, hr.QualificationStandLaeuftAb:
			return ""
		case hr.QualificationStandAbgelaufen:
			if s.GueltigBis != nil {
				return "Führerscheinkontrolle abgelaufen seit " + s.GueltigBis.Format("02.01.2006")
			}
			return "Führerscheinkontrolle abgelaufen"
		}
	}
	return "Keine gültige Führerscheinkontrolle erfasst"
}

// schedulesAt liefert die aktiven Termine des Fahrzeugs mit ihrem Stand am
// Tag day - nur mit Erledigungen bis zu diesem Tag (wie assets.standingAt):
// eine spaeter bestandene HU hebt eine Sperre am Zuordnungstag nicht auf.
func schedulesAt(ctx context.Context, tx pgx.Tx, vehicleID uuid.UUID, day time.Time) ([]ScheduleWithStanding, error) {
	rows, err := tx.Query(ctx, `SELECT `+scheduleColumns+` FROM fleet_vehicle_schedules s
		WHERE s.vehicle_id=$1 AND s.aktiv
		ORDER BY array_position(ARRAY['hu','uvv','wartung','versicherung','leasing','sonstige'], s.art), lower(s.bezeichnung), s.id`, vehicleID)
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
	completions, err := loadActiveCompletions(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ScheduleWithStanding, 0, len(schedules))
	for _, sc := range schedules {
		upTo := make([]Completion, 0)
		for _, c := range completions[sc.ID] {
			if !dateOnly(c.ErledigtAm).After(dateOnly(day)) {
				upTo = append(upTo, c)
			}
		}
		out = append(out, ScheduleWithStanding{Schedule: sc, Standing: EvaluateSchedule(sc.Art, sc.Starttermin, sc.StartKm, upTo, day, DefaultScheduleWarnDays)})
	}
	return out, nil
}

// Assign ordnet ein Fahrzeug einem Mitarbeiter zu.
//
// Zuerst wird die Fahrzeugzeile gesperrt - dieselbe Sperre wie beim
// Abmelden und beim Erfassen von Erledigungen. Damit kann zwischen den
// Pruefungen hier und dem Schreiben weder eine Abmeldung noch eine
// durchgefallene HU noch eine zweite Zuordnung dazwischenkommen. Dass
// hoechstens eine Zuordnung offen ist, garantiert zusaetzlich der Index
// aus Migration 102.
func (s *AssignmentService) Assign(ctx context.Context, in AssignInput, actorUserID, companyID string) (*Assignment, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.VehicleID == uuid.Nil {
		return nil, errors.New("vehicle_id erforderlich")
	}
	if in.EmployeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	erstzulassung, err := lockActiveVehicle(ctx, tx, in.VehicleID, companyID)
	if err != nil {
		return nil, err
	}
	if err := ValidateAssignmentStart(in.Beginn, erstzulassung, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	beginn := dateOnly(in.Beginn)

	schedules, err := schedulesAt(ctx, tx, in.VehicleID, beginn)
	if err != nil {
		return nil, err
	}
	if err := assignmentBlock(schedules); err != nil {
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

	// Offen ist offen - unabhaengig vom Datum.
	var offen bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fleet_vehicle_assignments WHERE vehicle_id=$1 AND ende IS NULL AND status='active')`, in.VehicleID).Scan(&offen); err != nil {
		return nil, err
	}
	if offen {
		return nil, errors.New("Fahrzeug ist bereits zugeordnet")
	}
	// Eine nachgetragene Zuordnung darf nicht in eine fruehere fallen; Ende
	// und neuer Beginn am selben Tag sind zulaessig. Wie bei assets.Issue
	// genuegt der Blick auf die Enddaten (offene sind oben abgelehnt, das
	// Ende liegt nie vor dem Beginn - CHECK in Migration 102).
	var overlap bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fleet_vehicle_assignments
		WHERE vehicle_id=$1 AND status='active' AND ende > $2)`, in.VehicleID, beginn).Scan(&overlap); err != nil {
		return nil, err
	}
	if overlap {
		return nil, errors.New("Zuordnung überschneidet sich mit einer früheren Zuordnung dieses Fahrzeugs")
	}

	// Fuehrerscheinkontrolle am Tag des Beginns - nur fuer den Hinweis.
	standings, err := s.quals.Standing(ctx, in.EmployeeID, beginn, hr.DefaultQualificationWarnDays, companyID)
	if err != nil {
		return nil, err
	}
	hints := scheduleHints(schedules)
	if h := licenseHint(standings); h != "" {
		hints = append(hints, h)
	}

	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO fleet_vehicle_assignments (id, vehicle_id, employee_id, beginn, notiz, zugeordnet_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		id, in.VehicleID, in.EmployeeID, beginn, strings.TrimSpace(in.Notiz), nullableUserID(actorUserID)); err != nil {
		if isUniqueViolation(err, oneOpenAssignmentIndex) {
			return nil, errors.New("Fahrzeug ist bereits zugeordnet")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	x, err := s.GetAssignment(ctx, id, companyID)
	if err != nil {
		return nil, err
	}
	x.Hinweise = hints
	return x, nil
}

func (s *AssignmentService) GetAssignment(ctx context.Context, id uuid.UUID, companyID string) (*Assignment, error) {
	x, err := scanAssignment(s.pg.QueryRow(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
		 WHERE x.id=$1 AND v.company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Zuordnung nicht gefunden")
		}
		return nil, err
	}
	return x, nil
}

// ListAssignments liefert Zuordnungen des Mandanten, die juengsten zuerst.
// Stornierte nur auf Wunsch; NurOffen schraenkt auf nicht beendete ein.
func (s *AssignmentService) ListAssignments(ctx context.Context, companyID string, f AssignmentFilter) ([]Assignment, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
		 WHERE v.company_id=$1
		   AND ($2::uuid IS NULL OR x.vehicle_id=$2)
		   AND ($3::uuid IS NULL OR x.employee_id=$3)
		   AND (NOT $4 OR x.ende IS NULL)
		   AND (x.status='active' OR $5)
		 ORDER BY x.beginn DESC, x.created_at DESC, x.id`,
		companyID, f.VehicleID, f.EmployeeID, f.NurOffen, f.IncludeStorniert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Assignment, 0)
	for rows.Next() {
		x, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

// ValidateAssignmentEnd prueft das Ende ohne Datenbank. Ende am Tag des
// Beginns ist zulaessig.
func ValidateAssignmentEnd(ende, beginn, today time.Time) error {
	if ende.IsZero() {
		return errors.New("Ende erforderlich")
	}
	if dateOnly(ende).After(dateOnly(today)) {
		return errors.New("Ende darf nicht in der Zukunft liegen")
	}
	if dateOnly(ende).Before(dateOnly(beginn)) {
		return errors.New("Ende darf nicht vor dem Beginn liegen")
	}
	return nil
}

// lockAssignment sperrt eine Zuordnung des Mandanten und liefert Status,
// Beginn und Ende.
func lockAssignment(ctx context.Context, tx pgx.Tx, id uuid.UUID, companyID string) (status string, beginn time.Time, ende *time.Time, err error) {
	err = tx.QueryRow(ctx, `
		SELECT x.status, x.beginn, x.ende
		  FROM fleet_vehicle_assignments x
		  JOIN fleet_vehicles v ON v.id = x.vehicle_id
		 WHERE x.id=$1 AND v.company_id=$2
		   FOR UPDATE OF x
	`, id, companyID).Scan(&status, &beginn, &ende)
	if errors.Is(err, pgx.ErrNoRows) {
		err = errors.New("Zuordnung nicht gefunden")
	}
	return
}

// End beendet eine offene Zuordnung (der Mitarbeiter gibt das Fahrzeug
// ab). Einmalig; eine stornierte Zuordnung kann nicht beendet werden.
func (s *AssignmentService) End(ctx context.Context, id uuid.UUID, ende time.Time, actorUserID, companyID string) (*Assignment, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Zuordnungs-ID erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	status, beginn, bereits, err := lockAssignment(ctx, tx, id, companyID)
	if err != nil {
		return nil, err
	}
	if status == AssignmentStorniert {
		return nil, errors.New("Zuordnung ist storniert")
	}
	if bereits != nil {
		return nil, errors.New("Zuordnung ist bereits beendet")
	}
	if err := ValidateAssignmentEnd(ende, beginn, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	ct, err := tx.Exec(ctx, `
		UPDATE fleet_vehicle_assignments SET ende=$2, beendet_by_user_id=$3
		 WHERE id=$1 AND status='active' AND ende IS NULL
	`, id, dateOnly(ende), nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Zuordnung ist bereits beendet")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAssignment(ctx, id, companyID)
}

// StornoAssignment storniert eine falsch erfasste Zuordnung - offen oder
// beendet, auch am abgemeldeten Fahrzeug (Berichtigung der Historie).
// Einmalig, mit Begruendung. Eine stornierte offene Zuordnung gibt das
// Fahrzeug wieder frei.
func (s *AssignmentService) StornoAssignment(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*Assignment, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Zuordnungs-ID erforderlich")
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

	status, _, _, err := lockAssignment(ctx, tx, id, companyID)
	if err != nil {
		return nil, err
	}
	if status == AssignmentStorniert {
		return nil, errors.New("Zuordnung ist bereits storniert")
	}
	ct, err := tx.Exec(ctx, `
		UPDATE fleet_vehicle_assignments SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3
		 WHERE id=$1 AND status='active'
	`, id, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Zuordnung ist bereits storniert")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAssignment(ctx, id, companyID)
}
