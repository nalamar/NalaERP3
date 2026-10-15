package assets

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/hr"
)

// Ausgabe und Ruecknahme von Betriebsmitteln (ADR 0028, Backlog F.3.4).
//
// Muster der Zeiteintraege (ADR 0024): eine Ausgabe wird nicht geaendert
// und nicht geloescht. Erlaubt sind genau zwei einmalige Uebergaenge -
// Rueckgabe und Storno. Die Historie "wer hatte die Leiter am
// Unfalltag?" bleibt vollstaendig.
//
// Ausgegeben wird an Mitarbeiter, ohne Empfangsbestaetigung (Entscheidung
// des Nutzers vom 2026-10-15). Die Sperre bei nicht bestandener Pruefung
// kommt mit F.3.5 dazu.

const (
	AssignmentActive    = "active"
	AssignmentStorniert = "storniert"
)

// Name des partiellen UNIQUE-Index aus Migration 089: hoechstens eine
// offene Ausgabe je Asset.
const oneOpenAssignmentIndex = "idx_asset_assignments_one_open"

type Assignment struct {
	ID                      uuid.UUID  `json:"id"`
	AssetID                 uuid.UUID  `json:"asset_id"`
	Inventarnummer          string     `json:"inventarnummer"`
	Bezeichnung             string     `json:"bezeichnung"`
	EmployeeID              uuid.UUID  `json:"employee_id"`
	MitarbeiterName         string     `json:"mitarbeiter_name"`
	AusgegebenAm            time.Time  `json:"ausgegeben_am"`
	ZurueckgegebenAm        *time.Time `json:"zurueckgegeben_am"`
	RueckgabeZustand        string     `json:"rueckgabe_zustand"`
	Notiz                   string     `json:"notiz"`
	Status                  string     `json:"status"`
	StornoReason            string     `json:"storno_reason,omitempty"`
	StorniertAt             *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID       *string    `json:"storniert_by_user_id,omitempty"`
	AusgegebenByUserID      *string    `json:"ausgegeben_by_user_id"`
	ZurueckgenommenByUserID *string    `json:"zurueckgenommen_by_user_id"`
	CreatedAt               time.Time  `json:"created_at"`
}

type IssueInput struct {
	AssetID      uuid.UUID `json:"asset_id"`
	EmployeeID   uuid.UUID `json:"employee_id"`
	AusgegebenAm time.Time `json:"ausgegeben_am"`
	Notiz        string    `json:"notiz"`
}

// AssignmentFilter schraenkt die Liste ein. Ohne Angaben: alle nicht
// stornierten Ausgaben des Mandanten.
type AssignmentFilter struct {
	AssetID          *uuid.UUID
	EmployeeID       *uuid.UUID
	NurOffen         bool
	IncludeStorniert bool
}

const assignmentColumns = `x.id, x.asset_id, a.inventarnummer, a.bezeichnung, x.employee_id,
	btrim(e.first_name || ' ' || e.last_name), x.ausgegeben_am, x.zurueckgegeben_am, x.rueckgabe_zustand, x.notiz,
	x.status, x.storno_reason, x.storniert_at, x.storniert_by_user_id, x.ausgegeben_by_user_id,
	x.zurueckgenommen_by_user_id, x.created_at`

// Der Mandant haengt am Asset; der Mitarbeiter gehoert beim Ausgeben
// nachweislich zum selben Mandanten.
const assignmentFrom = `
	  FROM asset_assignments x
	  JOIN assets a ON a.id = x.asset_id
	  JOIN hr_employees e ON e.id = x.employee_id`

func scanAssignment(row pgx.Row) (*Assignment, error) {
	var x Assignment
	if err := row.Scan(&x.ID, &x.AssetID, &x.Inventarnummer, &x.Bezeichnung, &x.EmployeeID,
		&x.MitarbeiterName, &x.AusgegebenAm, &x.ZurueckgegebenAm, &x.RueckgabeZustand, &x.Notiz,
		&x.Status, &x.StornoReason, &x.StorniertAt, &x.StorniertByUserID, &x.AusgegebenByUserID,
		&x.ZurueckgenommenByUserID, &x.CreatedAt); err != nil {
		return nil, err
	}
	return &x, nil
}

func nullableUserID(userID string) *string {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	return &userID
}

// ValidateIssueDate prueft das Ausgabedatum ohne Datenbank. Erfasst wird,
// was ausgegeben IST, nicht was geplant ist.
func ValidateIssueDate(ausgegebenAm time.Time, anschaffungsdatum *time.Time, today time.Time) error {
	if ausgegebenAm.IsZero() {
		return errors.New("Ausgabedatum erforderlich")
	}
	if dateOnly(ausgegebenAm).After(dateOnly(today)) {
		return errors.New("Ausgabedatum darf nicht in der Zukunft liegen")
	}
	if anschaffungsdatum != nil && dateOnly(ausgegebenAm).Before(dateOnly(*anschaffungsdatum)) {
		return errors.New("Ausgabedatum darf nicht vor dem Anschaffungsdatum liegen")
	}
	return nil
}

// ValidateReturnDate prueft das Rueckgabedatum ohne Datenbank. Rueckgabe
// am Ausgabetag ist zulaessig.
func ValidateReturnDate(zurueckgegebenAm, ausgegebenAm, today time.Time) error {
	if zurueckgegebenAm.IsZero() {
		return errors.New("Rückgabedatum erforderlich")
	}
	if dateOnly(zurueckgegebenAm).After(dateOnly(today)) {
		return errors.New("Rückgabedatum darf nicht in der Zukunft liegen")
	}
	if dateOnly(zurueckgegebenAm).Before(dateOnly(ausgegebenAm)) {
		return errors.New("Rückgabedatum darf nicht vor dem Ausgabedatum liegen")
	}
	return nil
}

// Issue gibt ein Asset an einen Mitarbeiter aus.
//
// Zuerst wird die Asset-Zeile gesperrt - DIESELBE Sperre wie in Retire.
// Damit kann zwischen den Pruefungen hier und dem Schreiben weder eine
// Ausmusterung noch eine zweite Ausgabe dazwischenkommen. Dass hoechstens
// eine Ausgabe offen ist, garantiert zusaetzlich der Index aus Migration
// 089.
func (s *Service) Issue(ctx context.Context, in IssueInput, actorUserID, companyID string) (*Assignment, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.AssetID == uuid.Nil {
		return nil, errors.New("asset_id erforderlich")
	}
	if in.EmployeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ausgemustertAm, anschaffungsdatum *time.Time
	if err := tx.QueryRow(ctx, `SELECT ausgemustert_am, anschaffungsdatum FROM assets WHERE id=$1 AND company_id=$2 FOR UPDATE`, in.AssetID, companyID).
		Scan(&ausgemustertAm, &anschaffungsdatum); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Asset nicht gefunden")
		}
		return nil, err
	}
	if ausgemustertAm != nil {
		return nil, errors.New("Asset ist ausgemustert")
	}
	if err := ValidateIssueDate(in.AusgegebenAm, anschaffungsdatum, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	ausgegebenAm := dateOnly(in.AusgegebenAm)

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
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM asset_assignments WHERE asset_id=$1 AND zurueckgegeben_am IS NULL AND status='active')`, in.AssetID).Scan(&offen); err != nil {
		return nil, err
	}
	if offen {
		return nil, errors.New("Asset ist bereits ausgegeben")
	}
	// Die Historie bleibt widerspruchsfrei: eine nachgetragene Ausgabe darf
	// nicht in den Zeitraum einer frueheren fallen. Rueckgabe und neue
	// Ausgabe am selben Tag sind zulaessig. Es genuegt, die Rueckgabedaten
	// zu pruefen: offene Ausgaben sind oben bereits abgelehnt, und bei
	// zurueckgegebenen liegt die Rueckgabe nie vor der Ausgabe (CHECK in
	// Migration 089) - eine spaetere Ausgabe hat also auch eine spaetere
	// Rueckgabe.
	var overlap bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM asset_assignments
		WHERE asset_id=$1 AND status='active' AND zurueckgegeben_am > $2)`, in.AssetID, ausgegebenAm).Scan(&overlap); err != nil {
		return nil, err
	}
	if overlap {
		return nil, errors.New("Ausgabe überschneidet sich mit einer früheren Ausgabe dieses Assets")
	}

	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO asset_assignments (id, asset_id, employee_id, ausgegeben_am, notiz, ausgegeben_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		id, in.AssetID, in.EmployeeID, ausgegebenAm, strings.TrimSpace(in.Notiz), nullableUserID(actorUserID)); err != nil {
		if isUniqueViolation(err, oneOpenAssignmentIndex) {
			return nil, errors.New("Asset ist bereits ausgegeben")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAssignment(ctx, id, companyID)
}

// lockAssignment sperrt eine Ausgabe des Mandanten und liefert Status,
// Ausgabe- und Rueckgabedatum.
func lockAssignment(ctx context.Context, tx pgx.Tx, id uuid.UUID, companyID string) (status string, ausgegebenAm time.Time, zurueckgegebenAm *time.Time, err error) {
	err = tx.QueryRow(ctx, `
		SELECT x.status, x.ausgegeben_am, x.zurueckgegeben_am
		  FROM asset_assignments x
		  JOIN assets a ON a.id = x.asset_id
		 WHERE x.id=$1 AND a.company_id=$2
		   FOR UPDATE OF x
	`, id, companyID).Scan(&status, &ausgegebenAm, &zurueckgegebenAm)
	if errors.Is(err, pgx.ErrNoRows) {
		err = errors.New("Ausgabe nicht gefunden")
	}
	return
}

// Return nimmt ein ausgegebenes Asset zurueck. Einmalig; eine stornierte
// Ausgabe kann nicht zurueckgegeben werden. Der Zustand ist Freitext
// ("in Ordnung", "Kabel beschaedigt", ...) und optional.
func (s *Service) Return(ctx context.Context, id uuid.UUID, zurueckgegebenAm time.Time, zustand, actorUserID, companyID string) (*Assignment, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Ausgabe-ID erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	status, ausgegebenAm, bereits, err := lockAssignment(ctx, tx, id, companyID)
	if err != nil {
		return nil, err
	}
	if status == AssignmentStorniert {
		return nil, errors.New("Ausgabe ist storniert")
	}
	if bereits != nil {
		return nil, errors.New("Asset wurde bereits zurückgegeben")
	}
	if err := ValidateReturnDate(zurueckgegebenAm, ausgegebenAm, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	ct, err := tx.Exec(ctx, `
		UPDATE asset_assignments SET zurueckgegeben_am=$2, rueckgabe_zustand=$3, zurueckgenommen_by_user_id=$4
		 WHERE id=$1 AND status='active' AND zurueckgegeben_am IS NULL
	`, id, dateOnly(zurueckgegebenAm), strings.TrimSpace(zustand), nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Asset wurde bereits zurückgegeben")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAssignment(ctx, id, companyID)
}

// StornoAssignment storniert eine falsch erfasste Ausgabe - offen oder
// zurueckgegeben. Einmalig, mit Begruendung. Eine stornierte offene
// Ausgabe gibt das Asset wieder frei.
func (s *Service) StornoAssignment(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*Assignment, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Ausgabe-ID erforderlich")
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
		return nil, errors.New("Ausgabe ist bereits storniert")
	}
	ct, err := tx.Exec(ctx, `
		UPDATE asset_assignments SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3
		 WHERE id=$1 AND status='active'
	`, id, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Ausgabe ist bereits storniert")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAssignment(ctx, id, companyID)
}

func (s *Service) GetAssignment(ctx context.Context, id uuid.UUID, companyID string) (*Assignment, error) {
	x, err := scanAssignment(s.pg.QueryRow(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
		 WHERE x.id=$1 AND a.company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Ausgabe nicht gefunden")
		}
		return nil, err
	}
	return x, nil
}

// ListAssignments liefert Ausgaben des Mandanten, die neuesten zuerst.
// Stornierte nur auf Wunsch; NurOffen schraenkt auf nicht zurueckgegebene
// ein.
func (s *Service) ListAssignments(ctx context.Context, companyID string, f AssignmentFilter) ([]Assignment, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
		 WHERE a.company_id=$1
		   AND ($2::uuid IS NULL OR x.asset_id=$2)
		   AND ($3::uuid IS NULL OR x.employee_id=$3)
		   AND (NOT $4 OR x.zurueckgegeben_am IS NULL)
		   AND (x.status='active' OR $5)
		 ORDER BY x.ausgegeben_am DESC, x.created_at DESC, x.id`,
		companyID, f.AssetID, f.EmployeeID, f.NurOffen, f.IncludeStorniert)
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
