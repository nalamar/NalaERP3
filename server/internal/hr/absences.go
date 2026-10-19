package hr

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Abwesenheiten und Krankmeldungen (ADR 0029, Backlog F.8.4.2).
//
// Eine Krankmeldung ist eine Abwesenheit vom Typ "sick" ohne Antrag. Sie
// traegt nur Zeitraum und Attest ja/nein - keine Diagnose, keinen Grund,
// keinen Freitext (Gesundheitsdaten, Art. 9 DSGVO).

// Absence ist eine Abwesenheit: aus einem genehmigten Urlaubsantrag oder
// eine Krankmeldung.
type Absence struct {
	ID                 uuid.UUID  `json:"id"`
	EmployeeID         uuid.UUID  `json:"employee_id"`
	Typ                string     `json:"typ"`
	StartDate          time.Time  `json:"start_date"`
	EndDate            time.Time  `json:"end_date"`
	Status             string     `json:"status"`
	SourceRequestID    *uuid.UUID `json:"source_request_id,omitempty"`
	Attest             bool       `json:"attest"`
	GemeldetVon        string     `json:"gemeldet_von"`
	GemeldetByUserID   *string    `json:"gemeldet_by_user_id,omitempty"`
	BestaetigtAt       *time.Time `json:"bestaetigt_at,omitempty"`
	BestaetigtByUserID *string    `json:"bestaetigt_by_user_id,omitempty"`
	StorniertAt        *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID  *string    `json:"storniert_by_user_id,omitempty"`
	StornoReason       string     `json:"storno_reason,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

const absenceColumns = `a.id, a.employee_id, a.typ, a.start_date, a.end_date, a.status, a.source_request_id, a.attest, a.gemeldet_von,
	a.gemeldet_by_user_id, a.bestaetigt_at, a.bestaetigt_by_user_id, a.storniert_at, a.storniert_by_user_id, COALESCE(a.storno_reason,''), a.created_at`

func scanAbsence(row pgx.Row) (*Absence, error) {
	var a Absence
	if err := row.Scan(&a.ID, &a.EmployeeID, &a.Typ, &a.StartDate, &a.EndDate, &a.Status, &a.SourceRequestID, &a.Attest, &a.GemeldetVon,
		&a.GemeldetByUserID, &a.BestaetigtAt, &a.BestaetigtByUserID, &a.StorniertAt, &a.StorniertByUserID, &a.StornoReason, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// SickReport ist eine Krankmeldung. Attest wird bei einer Selbstmeldung
// ignoriert - ob ein Attest vorliegt, haelt die Personalabteilung fest.
type SickReport struct {
	EmployeeID uuid.UUID
	StartDate  time.Time
	EndDate    time.Time
	Attest     bool
}

type AbsenceService struct{ pg *pgxpool.Pool }

func NewAbsenceService(pg *pgxpool.Pool) *AbsenceService { return &AbsenceService{pg: pg} }

// Get liefert eine Abwesenheit des Mandanten.
func (s *AbsenceService) Get(ctx context.Context, id uuid.UUID, companyID string) (*Absence, error) {
	a, err := scanAbsence(s.pg.QueryRow(ctx, `SELECT `+absenceColumns+` FROM hr_absences a
		JOIN hr_employees e ON e.id = a.employee_id WHERE a.id=$1 AND e.company_id=$2`, id, companyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Abwesenheit nicht gefunden")
	}
	return a, err
}

// ReportSick erfasst eine Krankmeldung. bySelf: der Mitarbeiter meldet
// sich selbst (ohne Attest, noch nicht bestaetigt); sonst erfasst die
// Personalabteilung (gilt als bestaetigt). Krankmeldungen desselben
// Mitarbeiters duerfen sich nicht ueberschneiden.
func (s *AbsenceService) ReportSick(ctx context.Context, in SickReport, bySelf bool, actorUserID, companyID string) (*Absence, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.EmployeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	if in.StartDate.IsZero() || in.EndDate.IsZero() {
		return nil, errors.New("start/end erforderlich")
	}
	start, end := dateOnly(in.StartDate), dateOnly(in.EndDate)
	if end.Before(start) {
		return nil, errors.New("Enddatum darf nicht vor Startdatum liegen")
	}
	if !validCalendarYear(start.Year()) || !validCalendarYear(end.Year()) {
		return nil, errCalendarYear
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// Sperre auf den Mitarbeiter: zwei gleichzeitige Meldungen koennen sich
	// so nicht beide an der Ueberschneidungspruefung vorbeischieben.
	var locked uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM hr_employees WHERE id=$1 AND company_id=$2 FOR UPDATE`, in.EmployeeID, companyID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Mitarbeiter nicht gefunden")
	}
	if err != nil {
		return nil, err
	}
	var overlapping bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_absences WHERE employee_id=$1 AND typ='sick' AND status='approved'
		AND start_date <= $3 AND end_date >= $2)`, in.EmployeeID, start, end).Scan(&overlapping); err != nil {
		return nil, err
	}
	if overlapping {
		return nil, errors.New("Zeitraum überschneidet sich mit einer bestehenden Krankmeldung")
	}
	id := uuid.New()
	von, attest := "personal", in.Attest
	var confirmedAt, confirmedBy any = time.Now(), nullableUserID(actorUserID)
	if bySelf {
		von, attest, confirmedAt, confirmedBy = "selbst", false, nil, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO hr_absences (id, employee_id, typ, start_date, end_date, status, attest, gemeldet_von, gemeldet_by_user_id, bestaetigt_at, bestaetigt_by_user_id)
		VALUES ($1,$2,'sick',$3,$4,'approved',$5,$6,$7,$8,$9)`,
		id, in.EmployeeID, start, end, attest, von, nullableUserID(actorUserID), confirmedAt, confirmedBy); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// loadActiveSick laedt eine geltende Krankmeldung zur Aenderung.
func (s *AbsenceService) loadActiveSick(ctx context.Context, id uuid.UUID, companyID string) (*Absence, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	a, err := s.Get(ctx, id, companyID)
	if err != nil {
		return nil, err
	}
	if a.Typ != "sick" || a.SourceRequestID != nil {
		return nil, errors.New("Ungültige Abwesenheit: keine Krankmeldung - Urlaub wird über den Urlaubsantrag storniert")
	}
	if a.Status == "storniert" {
		return nil, errors.New("Krankmeldung ist bereits storniert")
	}
	return a, nil
}

// SetAttest haelt fest, ob ein Attest vorliegt. Mit Attest zaehlen kranke
// Arbeitstage im genehmigten Urlaub nicht als genommen (§9 BUrlG).
func (s *AbsenceService) SetAttest(ctx context.Context, id uuid.UUID, attest bool, companyID string) (*Absence, error) {
	if _, err := s.loadActiveSick(ctx, id, companyID); err != nil {
		return nil, err
	}
	if _, err := s.pg.Exec(ctx, `UPDATE hr_absences SET attest=$2 WHERE id=$1 AND status='approved'`, id, attest); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// Confirm bestaetigt eine Selbstmeldung: sie verschwindet aus der Liste
// "neue Krankmeldungen".
func (s *AbsenceService) Confirm(ctx context.Context, id uuid.UUID, actorUserID, companyID string) (*Absence, error) {
	if _, err := s.loadActiveSick(ctx, id, companyID); err != nil {
		return nil, err
	}
	ct, err := s.pg.Exec(ctx, `UPDATE hr_absences SET bestaetigt_at=now(), bestaetigt_by_user_id=$2
		WHERE id=$1 AND status='approved' AND bestaetigt_at IS NULL`, id, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, errors.New("Krankmeldung ist bereits bestätigt")
	}
	return s.Get(ctx, id, companyID)
}

// Storno storniert eine Krankmeldung mit Begruendung (z. B. falsch
// gemeldet). Abwesenheiten aus Urlaubsantraegen werden ueber den Antrag
// storniert (LeaveService.Storno).
func (s *AbsenceService) Storno(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*Absence, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("Begründung erforderlich")
	}
	if _, err := s.loadActiveSick(ctx, id, companyID); err != nil {
		return nil, err
	}
	ct, err := s.pg.Exec(ctx, `UPDATE hr_absences SET status='storniert', storniert_at=now(), storniert_by_user_id=$2, storno_reason=$3
		WHERE id=$1 AND status='approved'`, id, nullableUserID(actorUserID), reason)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, errors.New("Krankmeldung ist bereits storniert")
	}
	return s.Get(ctx, id, companyID)
}

// ListNewSick liefert die selbst gemeldeten, noch nicht bestaetigten und
// nicht stornierten Krankmeldungen des Mandanten, aelteste zuerst.
func (s *AbsenceService) ListNewSick(ctx context.Context, companyID string) ([]Absence, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	rows, err := s.pg.Query(ctx, `SELECT `+absenceColumns+` FROM hr_absences a JOIN hr_employees e ON e.id = a.employee_id
		WHERE e.company_id=$1 AND a.typ='sick' AND a.gemeldet_von='selbst' AND a.bestaetigt_at IS NULL AND a.status='approved'
		ORDER BY a.created_at, a.id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Absence{}
	for rows.Next() {
		a, err := scanAbsence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// AbsenceEntry ist eine Zeile der Abwesenheitsuebersicht: die Abwesenheit
// mit dem Namen des Mitarbeiters.
type AbsenceEntry struct {
	Absence
	PersonalNr *string `json:"personalnummer,omitempty"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
}

// AbsenceFilter waehlt die Abwesenheiten der Uebersicht. From und To sind
// Pflicht (beide einschliesslich); gezeigt wird jede Abwesenheit, die den
// Zeitraum beruehrt.
type AbsenceFilter struct {
	From, To         time.Time
	EmployeeID       *uuid.UUID
	TeamID           *uuid.UUID
	IncludeStorniert bool
}

// maxAbsenceRangeDays begrenzt die Uebersicht auf ein Jahr - genug fuer
// jede Planung, und eine Abfrage ueber Jahrzehnte bleibt aus.
const maxAbsenceRangeDays = 366

// List liefert die Abwesenheiten im Zeitraum, nach Beginn und Namen
// sortiert. Welche Mitarbeiter der Aufrufer sehen darf, entscheidet die
// Route (eigene mit leave_requests.self, alle mit .read).
func (s *AbsenceService) List(ctx context.Context, f AbsenceFilter, companyID string) ([]AbsenceEntry, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if f.From.IsZero() || f.To.IsZero() {
		return nil, errors.New("from/to erforderlich")
	}
	from, to := dateOnly(f.From), dateOnly(f.To)
	if to.Before(from) {
		return nil, errors.New("Enddatum darf nicht vor Startdatum liegen")
	}
	if to.Sub(from).Hours()/24+1 > maxAbsenceRangeDays {
		return nil, errors.New("Ungültiger Zeitraum: höchstens 366 Tage")
	}
	where := `e.company_id=$1 AND a.start_date <= $3 AND a.end_date >= $2`
	args := []any{companyID, from, to}
	if !f.IncludeStorniert {
		where += ` AND a.status='approved'`
	}
	if f.EmployeeID != nil {
		args = append(args, *f.EmployeeID)
		where += ` AND a.employee_id=$` + itoa(len(args))
	}
	if f.TeamID != nil {
		args = append(args, *f.TeamID)
		where += ` AND e.team_id=$` + itoa(len(args))
	}
	rows, err := s.pg.Query(ctx, `SELECT `+absenceColumns+`, e.personalnummer, e.first_name, e.last_name
		FROM hr_absences a JOIN hr_employees e ON e.id = a.employee_id
		WHERE `+where+` ORDER BY a.start_date, e.last_name, e.first_name, a.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AbsenceEntry{}
	for rows.Next() {
		var x AbsenceEntry
		a := &x.Absence
		if err := rows.Scan(&a.ID, &a.EmployeeID, &a.Typ, &a.StartDate, &a.EndDate, &a.Status, &a.SourceRequestID, &a.Attest, &a.GemeldetVon,
			&a.GemeldetByUserID, &a.BestaetigtAt, &a.BestaetigtByUserID, &a.StorniertAt, &a.StorniertByUserID, &a.StornoReason, &a.CreatedAt,
			&x.PersonalNr, &x.FirstName, &x.LastName); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
