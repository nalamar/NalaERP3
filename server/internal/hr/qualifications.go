package hr

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QualificationService erfasst, storniert und liest Nachweise (ADR 0027,
// Backlog F.2.4.2).
//
// Nachweise sind unveraenderlich: sie haben Beweisfunktion. Dieser Service
// kennt genau EINEN UPDATE - das einmalige Stornieren - und keinen DELETE.
// Ein Fehler wird storniert und neu erfasst; eine Wiederholung ist ein
// neuer Nachweis. TestQualificationServiceHasNoOtherWritePaths wacht
// darueber.
type QualificationService struct{ pg *pgxpool.Pool }

func NewQualificationService(pg *pgxpool.Pool) *QualificationService {
	return &QualificationService{pg: pg}
}

// QualificationInput beschreibt einen Nachweis fuer einen oder mehrere
// Mitarbeiter. Unterwiesen wird in der Praxis die ganze Werkstatt auf
// einmal; ein einzelner Mitarbeiter ist eine Gruppe aus einem.
type QualificationInput struct {
	EmployeeIDs []uuid.UUID `json:"employee_ids"`
	TypeID      uuid.UUID   `json:"type_id"`
	ErworbenAm  time.Time   `json:"erworben_am"`
	// GueltigBis: ausdruecklich angegeben (das Zertifikat nennt sein
	// eigenes Datum) oder nil - dann wird es aus dem Erwerbsdatum und der
	// Frist der Art berechnet; hat die Art keine Frist, ist der Nachweis
	// unbefristet.
	GueltigBis *time.Time `json:"gueltig_bis"`
	Aussteller string     `json:"aussteller"`
	NachweisNr string     `json:"nachweis_nr"`
	Note       string     `json:"note"`
}

const qualificationColumns = `q.id, q.employee_id, q.type_id, q.erworben_am, q.gueltig_bis, q.aussteller, q.nachweis_nr, q.note, q.status, q.storno_reason, q.storniert_at, q.storniert_by_user_id, q.created_by_user_id, q.created_at`

func scanQualification(row pgx.Row) (*EmployeeQualification, error) {
	var q EmployeeQualification
	if err := row.Scan(&q.ID, &q.EmployeeID, &q.TypeID, &q.ErworbenAm, &q.GueltigBis, &q.Aussteller, &q.NachweisNr, &q.Note, &q.Status, &q.StornoReason, &q.StorniertAt, &q.StorniertByUserID, &q.CreatedByUserID, &q.CreatedAt); err != nil {
		return nil, err
	}
	return &q, nil
}

// uniqueEmployeeIDs entfernt Doppelte und sortiert. Die Sortierung legt
// die Reihenfolge fest, in der die Mitarbeiterzeilen gesperrt werden -
// zwei gleichzeitige Gruppenerfassungen mit ueberlappenden Mitarbeitern
// sperren so in derselben Reihenfolge und koennen sich nicht gegenseitig
// blockieren.
func uniqueEmployeeIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// Create erfasst einen Nachweis fuer jeden genannten Mitarbeiter - in
// EINER Transaktion, ganz oder gar nicht. Es entsteht je Mitarbeiter ein
// eigener Nachweis; ein doppelt genannter Mitarbeiter bekommt einen.
//
// actorUserID ist der angemeldete Benutzer, der erfasst (leer =
// unbekannt/System).
func (s *QualificationService) Create(ctx context.Context, in QualificationInput, actorUserID, companyID string) ([]EmployeeQualification, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if len(in.EmployeeIDs) == 0 {
		return nil, errors.New("Mindestens ein Mitarbeiter erforderlich")
	}
	for _, id := range in.EmployeeIDs {
		if id == uuid.Nil {
			return nil, errors.New("employee_id erforderlich")
		}
	}
	if in.TypeID == uuid.Nil {
		return nil, errors.New("type_id erforderlich")
	}
	// "Heute" ist der Kalendertag in Europe/Berlin, wie bei den
	// Zeiteintraegen.
	if err := ValidateQualificationDates(in.ErworbenAm, in.GueltigBis, WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	employeeIDs := uniqueEmployeeIDs(in.EmployeeIDs)
	erworbenAm := dateOnly(in.ErworbenAm)

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Die Art muss zum Mandanten gehoeren und aktiv sein. Eine stillgelegte
	// Art nimmt keine NEUEN Nachweise an; ihre bestehenden bleiben.
	var aktiv bool
	var gueltigkeitMonate *int
	if err := tx.QueryRow(ctx, `SELECT aktiv, gueltigkeit_monate FROM hr_qualification_types WHERE id=$1 AND company_id=$2`, in.TypeID, companyID).Scan(&aktiv, &gueltigkeitMonate); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Qualifikationsart nicht gefunden")
		}
		return nil, err
	}
	if !aktiv {
		return nil, errors.New("Qualifikationsart ist stillgelegt")
	}

	// gueltig_bis wird HIER festgelegt und gespeichert. Eine spaetere
	// Aenderung der Frist an der Art aendert diesen Nachweis nicht.
	gueltigBis := in.GueltigBis
	if gueltigBis == nil {
		gueltigBis = QualificationValidUntil(erworbenAm, gueltigkeitMonate)
	} else {
		d := dateOnly(*gueltigBis)
		gueltigBis = &d
	}

	// Die Mitarbeiterzeilen werden gesperrt (in fester Reihenfolge). Das
	// prueft die Mandantenzugehoerigkeit JEDES Mitarbeiters und
	// serialisiert gleichzeitige Erfassungen fuer denselben Mitarbeiter,
	// damit die Doppelpruefung unten nicht von zwei Aufrufen zugleich
	// passiert wird.
	rows, err := tx.Query(ctx, `SELECT id FROM hr_employees WHERE id = ANY($1) AND company_id=$2 ORDER BY id FOR UPDATE`, employeeIDs, companyID)
	if err != nil {
		return nil, err
	}
	found := 0
	for rows.Next() {
		found++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if found != len(employeeIDs) {
		return nil, errors.New("Mitarbeiter nicht gefunden")
	}

	// Derselbe Nachweis ein zweites Mal (gleicher Mitarbeiter, gleiche
	// Art, gleicher Tag) ist ein Doppelklick oder eine doppelte Liste,
	// keine Wiederholung - eine Wiederholung hat ein anderes Datum.
	var duplicate bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM hr_employee_qualifications
			 WHERE employee_id = ANY($1) AND type_id=$2 AND erworben_am=$3 AND status='active'
		)`, employeeIDs, in.TypeID, erworbenAm).Scan(&duplicate); err != nil {
		return nil, err
	}
	if duplicate {
		return nil, errors.New("Nachweis für diesen Tag ist bereits erfasst")
	}

	out := make([]EmployeeQualification, 0, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		q, err := scanQualification(tx.QueryRow(ctx, `
			INSERT INTO hr_employee_qualifications AS q (id, employee_id, type_id, erworben_am, gueltig_bis, aussteller, nachweis_nr, note, created_by_user_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING `+qualificationColumns,
			uuid.New(), employeeID, in.TypeID, erworbenAm, gueltigBis, in.Aussteller, in.NachweisNr, in.Note, nullableUserID(actorUserID)))
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// Storno storniert einen Nachweis. Der Uebergang ist einmalig, und er
// schreibt ausschliesslich die vier Storno-Spalten: Erwerbsdatum,
// Gueltigkeit und alle uebrigen Angaben bleiben nachlesbar.
func (s *QualificationService) Storno(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*EmployeeQualification, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Nachweis-ID erforderlich")
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
		SELECT q.status
		  FROM hr_employee_qualifications q
		  JOIN hr_employees e ON e.id = q.employee_id
		 WHERE q.id=$1 AND e.company_id=$2
		   FOR UPDATE OF q
	`, id, companyID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Nachweis nicht gefunden")
		}
		return nil, err
	}
	if status == QualificationRecordStorniert {
		return nil, errors.New("Nachweis ist bereits storniert")
	}
	ct, err := tx.Exec(ctx, `
		UPDATE hr_employee_qualifications SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3
		 WHERE id=$1 AND status='active'
	`, id, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Nachweis ist bereits storniert")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

func (s *QualificationService) Get(ctx context.Context, id uuid.UUID, companyID string) (*EmployeeQualification, error) {
	q, err := scanQualification(s.pg.QueryRow(ctx, `
		SELECT `+qualificationColumns+`
		  FROM hr_employee_qualifications q
		  JOIN hr_employees e ON e.id = q.employee_id
		 WHERE q.id=$1 AND e.company_id=$2
	`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Nachweis nicht gefunden")
		}
		return nil, err
	}
	return q, nil
}

// ListForEmployee liefert die Nachweise eines Mitarbeiters, die juengsten
// zuerst. Stornierte nur mit includeStorniert - sie gehoeren zur
// Nachvollziehbarkeit, nicht in die Standardansicht.
func (s *QualificationService) ListForEmployee(ctx context.Context, employeeID uuid.UUID, includeStorniert bool, companyID string) ([]EmployeeQualification, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if employeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	var employeeOwned bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, employeeID, companyID).Scan(&employeeOwned); err != nil {
		return nil, err
	}
	if !employeeOwned {
		return nil, errors.New("Mitarbeiter nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `
		SELECT `+qualificationColumns+`
		  FROM hr_employee_qualifications q
		 WHERE q.employee_id=$1 AND (q.status='active' OR $2)
		 ORDER BY q.erworben_am DESC, q.created_at DESC, q.id
	`, employeeID, includeStorniert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EmployeeQualification, 0)
	for rows.Next() {
		q, err := scanQualification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}
