package hr

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Zwischenbestaetigungen eines Nachweises (Backlog F.10.3.2b, Migration
// 101): z. B. halbjaehrlich bei der Schweisserpruefung nach
// DIN EN ISO 9606-1. Wie die Nachweise selbst unveraenderlich - genau EIN
// UPDATE (das einmalige Stornieren), kein DELETE;
// TestQualificationConfirmationsHaveNoOtherWritePaths wacht darueber.
//
// Erfasst wird, was bestaetigt WURDE. Ob eine Bestaetigung rechtzeitig
// war, entscheidet die Fachregel beim Lesen (QualificationConfirmationDue);
// eine verspaetete wird daher nicht abgewiesen, sie zaehlt nur nicht.

// QualificationConfirmation entspricht einer Zeile in
// hr_qualification_confirmations.
type QualificationConfirmation struct {
	ID                uuid.UUID  `json:"id"`
	QualificationID   uuid.UUID  `json:"qualification_id"`
	BestaetigtAm      time.Time  `json:"bestaetigt_am"`
	BestaetigtDurch   string     `json:"bestaetigt_durch"`
	Note              string     `json:"note"`
	Status            string     `json:"status"`
	StornoReason      string     `json:"storno_reason,omitempty"`
	StorniertAt       *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID *string    `json:"storniert_by_user_id,omitempty"`
	CreatedByUserID   *string    `json:"created_by_user_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// QualificationConfirmationInput beschreibt eine Bestaetigung.
type QualificationConfirmationInput struct {
	BestaetigtAm time.Time `json:"bestaetigt_am"`
	// BestaetigtDurch: Name bzw. Funktion, z. B. "Schweissaufsicht".
	BestaetigtDurch string `json:"bestaetigt_durch"`
	Note            string `json:"note"`
}

const qualificationConfirmationColumns = `c.id, c.qualification_id, c.bestaetigt_am, c.bestaetigt_durch, c.note, c.status, c.storno_reason, c.storniert_at, c.storniert_by_user_id, c.created_by_user_id, c.created_at`

func scanQualificationConfirmation(row pgx.Row) (*QualificationConfirmation, error) {
	var c QualificationConfirmation
	if err := row.Scan(&c.ID, &c.QualificationID, &c.BestaetigtAm, &c.BestaetigtDurch, &c.Note, &c.Status, &c.StornoReason, &c.StorniertAt, &c.StorniertByUserID, &c.CreatedByUserID, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// ValidateQualificationConfirmation prueft eine Bestaetigung gegen ihren
// Nachweis. today wird uebergeben, damit die Regel testbar ist.
func ValidateQualificationConfirmation(in QualificationConfirmationInput, erworbenAm time.Time, gueltigBis *time.Time, today time.Time) error {
	if in.BestaetigtAm.IsZero() {
		return errors.New("Bestätigungsdatum erforderlich")
	}
	if strings.TrimSpace(in.BestaetigtDurch) == "" {
		return errors.New("Bestätigt durch erforderlich")
	}
	am := dateOnly(in.BestaetigtAm)
	if am.After(dateOnly(today)) {
		return errors.New("Bestätigungsdatum darf nicht in der Zukunft liegen")
	}
	if am.Before(dateOnly(erworbenAm)) {
		return errors.New("Bestätigungsdatum darf nicht vor dem Erwerbsdatum liegen")
	}
	// Nach dem Ablauf des Zertifikats gibt es nichts mehr zu bestaetigen;
	// dann ist ein neuer Nachweis faellig.
	if gueltigBis != nil && am.After(dateOnly(*gueltigBis)) {
		return errors.New("Bestätigungsdatum darf nicht nach dem Gültig-bis liegen")
	}
	return nil
}

// CreateConfirmation erfasst eine Bestaetigung zu einem Nachweis. Der
// Nachweis muss zum Mandanten gehoeren und aktiv sein, und seine Art muss
// eine Zwischenbestaetigung vorsehen.
func (s *QualificationService) CreateConfirmation(ctx context.Context, qualificationID uuid.UUID, in QualificationConfirmationInput, actorUserID, companyID string) (*QualificationConfirmation, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if qualificationID == uuid.Nil {
		return nil, errors.New("Nachweis-ID erforderlich")
	}
	if in.BestaetigtAm.IsZero() {
		return nil, errors.New("Bestätigungsdatum erforderlich")
	}
	if strings.TrimSpace(in.BestaetigtDurch) == "" {
		return nil, errors.New("Bestätigt durch erforderlich")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Die Nachweiszeile wird gesperrt: ein gleichzeitiges Stornieren des
	// Nachweises und eine doppelte Erfassung warten aufeinander.
	var status string
	var erworbenAm time.Time
	var gueltigBis *time.Time
	var bestaetigungMonate *int
	if err := tx.QueryRow(ctx, `
		SELECT q.status, q.erworben_am, q.gueltig_bis, t.bestaetigung_monate
		  FROM hr_employee_qualifications q
		  JOIN hr_employees e ON e.id = q.employee_id
		  JOIN hr_qualification_types t ON t.id = q.type_id
		 WHERE q.id=$1 AND e.company_id=$2
		   FOR UPDATE OF q
	`, qualificationID, companyID).Scan(&status, &erworbenAm, &gueltigBis, &bestaetigungMonate); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Nachweis nicht gefunden")
		}
		return nil, err
	}
	if status != QualificationRecordActive {
		return nil, errors.New("Nachweis ist storniert")
	}
	if bestaetigungMonate == nil {
		return nil, errors.New("Qualifikationsart sieht keine Zwischenbestätigung vor")
	}
	// "Heute" ist der Kalendertag in Europe/Berlin, wie bei den Nachweisen.
	if err := ValidateQualificationConfirmation(in, erworbenAm, gueltigBis, WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	am := dateOnly(in.BestaetigtAm)

	var duplicate bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM hr_qualification_confirmations
			 WHERE qualification_id=$1 AND bestaetigt_am=$2 AND status='active'
		)`, qualificationID, am).Scan(&duplicate); err != nil {
		return nil, err
	}
	if duplicate {
		return nil, errors.New("Bestätigung für diesen Tag ist bereits erfasst")
	}

	c, err := scanQualificationConfirmation(tx.QueryRow(ctx, `
		INSERT INTO hr_qualification_confirmations AS c (id, qualification_id, bestaetigt_am, bestaetigt_durch, note, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING `+qualificationConfirmationColumns,
		uuid.New(), qualificationID, am, strings.TrimSpace(in.BestaetigtDurch), in.Note, nullableUserID(actorUserID)))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// StornoConfirmation storniert eine Bestaetigung - einmalig, nur die vier
// Storno-Spalten. Danach zaehlt sie im Stand nicht mehr.
func (s *QualificationService) StornoConfirmation(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*QualificationConfirmation, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Bestätigungs-ID erforderlich")
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
		  FROM hr_qualification_confirmations c
		  JOIN hr_employee_qualifications q ON q.id = c.qualification_id
		  JOIN hr_employees e ON e.id = q.employee_id
		 WHERE c.id=$1 AND e.company_id=$2
		   FOR UPDATE OF c
	`, id, companyID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Bestätigung nicht gefunden")
		}
		return nil, err
	}
	if status == QualificationRecordStorniert {
		return nil, errors.New("Bestätigung ist bereits storniert")
	}
	ct, err := tx.Exec(ctx, `
		UPDATE hr_qualification_confirmations SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3
		 WHERE id=$1 AND status='active'
	`, id, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Bestätigung ist bereits storniert")
	}
	c, err := scanQualificationConfirmation(tx.QueryRow(ctx, `SELECT `+qualificationConfirmationColumns+` FROM hr_qualification_confirmations c WHERE c.id=$1`, id))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// ListConfirmations liefert die Bestaetigungen eines Nachweises, die
// juengsten zuerst; stornierte nur mit includeStorniert.
func (s *QualificationService) ListConfirmations(ctx context.Context, qualificationID uuid.UUID, includeStorniert bool, companyID string) ([]QualificationConfirmation, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if qualificationID == uuid.Nil {
		return nil, errors.New("Nachweis-ID erforderlich")
	}
	var owned bool
	if err := s.pg.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM hr_employee_qualifications q
			  JOIN hr_employees e ON e.id = q.employee_id
			 WHERE q.id=$1 AND e.company_id=$2
		)`, qualificationID, companyID).Scan(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Nachweis nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `
		SELECT `+qualificationConfirmationColumns+`
		  FROM hr_qualification_confirmations c
		 WHERE c.qualification_id=$1 AND (c.status='active' OR $2)
		 ORDER BY c.bestaetigt_am DESC, c.created_at DESC, c.id
	`, qualificationID, includeStorniert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]QualificationConfirmation, 0)
	for rows.Next() {
		c, err := scanQualificationConfirmation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}
