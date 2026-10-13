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

// TimeEntryService schreibt und liest Zeiteintraege (ADR 0024).
//
// Eintraege sind unveraenderlich. Dieser Service kennt genau zwei UPDATEs,
// jeder ist ein einmaliger Zustandsuebergang: das Beenden eines laufenden
// Eintrags (Stop) und das Stornieren (Storno, auch als Teil von Correct).
// Die Zeiten eines abgeschlossenen Eintrags aendert kein Pfad. Alles
// andere ist INSERT. Es gibt keinen DELETE.
// TestTimeEntryServiceHasNoOtherWritePaths wacht darueber.
type TimeEntryService struct{ pg *pgxpool.Pool }

func NewTimeEntryService(pg *pgxpool.Pool) *TimeEntryService { return &TimeEntryService{pg: pg} }

// TimeEntryInput beschreibt einen neuen Eintrag. Ohne EndedAt wird ein
// laufender Eintrag angelegt (Stempeln), mit EndedAt ein abgeschlossener
// (Nachtrag).
type TimeEntryInput struct {
	EmployeeID   uuid.UUID  `json:"employee_id"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	BreakMinutes int        `json:"break_minutes"`
	ProjectID    *uuid.UUID `json:"project_id,omitempty"`
	Note         string     `json:"note"`
}

const timeEntryColumns = `t.id, t.employee_id, t.work_date, t.started_at, t.ended_at, t.break_minutes, t.project_id, t.note, t.status, t.storno_reason, t.storniert_at, t.storniert_by_user_id, t.replaces_entry_id, t.created_by_user_id, t.created_at`

func scanTimeEntry(row pgx.Row) (*TimeEntry, error) {
	var e TimeEntry
	if err := row.Scan(&e.ID, &e.EmployeeID, &e.WorkDate, &e.StartedAt, &e.EndedAt, &e.BreakMinutes, &e.ProjectID, &e.Note, &e.Status, &e.StornoReason, &e.StorniertAt, &e.StorniertByUserID, &e.ReplacesEntryID, &e.CreatedByUserID, &e.CreatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}

func newTimeEntry(in TimeEntryInput) TimeEntry {
	return TimeEntry{
		ID:           uuid.New(),
		EmployeeID:   in.EmployeeID,
		WorkDate:     WorkDateOf(in.StartedAt),
		StartedAt:    in.StartedAt,
		EndedAt:      in.EndedAt,
		BreakMinutes: in.BreakMinutes,
		ProjectID:    in.ProjectID,
		Note:         in.Note,
		Status:       TimeEntryStatusActive,
	}
}

func nullableUserID(userID string) *string {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	return &userID
}

// Create legt einen Eintrag an. actorUserID ist der angemeldete Benutzer,
// der die Buchung vornimmt (leer = unbekannt/System) - das muss nicht der
// Mitarbeiter selbst sein.
func (s *TimeEntryService) Create(ctx context.Context, in TimeEntryInput, actorUserID, companyID string) (*TimeEntry, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.EmployeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	entry := newTimeEntry(in)
	if err := ValidateTimeEntry(entry, time.Now()); err != nil {
		return nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := insertTimeEntryTx(ctx, tx, entry, nil, actorUserID, companyID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, entry.ID, companyID)
}

// insertTimeEntryTx prueft Mandant, Projekt und Ueberschneidung und
// schreibt den Eintrag - innerhalb einer Transaktion des Aufrufers, damit
// eine Korrektur Storno und Ersatzeintrag gemeinsam festschreiben oder
// gemeinsam verwerfen kann. Der Eintrag muss ValidateTimeEntry bereits
// bestanden haben. replaces ist der bei einer Korrektur ersetzte Eintrag.
func insertTimeEntryTx(ctx context.Context, tx pgx.Tx, entry TimeEntry, replaces *uuid.UUID, actorUserID, companyID string) error {
	// Die Zeile des Mitarbeiters wird gesperrt. Das prueft die
	// Mandantenzugehoerigkeit UND serialisiert alle Buchungen fuer
	// denselben Mitarbeiter: ohne die Sperre koennten zwei gleichzeitige
	// Buchungen die Ueberschneidungspruefung unten beide passieren.
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM hr_employees WHERE id=$1 AND company_id=$2 FOR UPDATE`, entry.EmployeeID, companyID).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("Mitarbeiter nicht gefunden")
		}
		return err
	}

	if entry.ProjectID != nil {
		var projectOwned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND company_id=$2)`, *entry.ProjectID, companyID).Scan(&projectOwned); err != nil {
			return err
		}
		if !projectOwned {
			return errors.New("Projekt nicht gefunden")
		}
	}

	if entry.EndedAt == nil {
		var running bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_time_entries WHERE employee_id=$1 AND status='active' AND ended_at IS NULL)`, entry.EmployeeID).Scan(&running); err != nil {
			return err
		}
		if running {
			return errors.New("Es läuft bereits ein Zeiteintrag")
		}
	}

	// Ueberschneidung mit einem anderen aktiven Eintrag. Zeitraeume sind
	// halboffen [Beginn, Ende): ein Eintrag, der genau dort beginnt, wo
	// ein anderer endet, ueberschneidet sich nicht. Ein laufender Eintrag
	// reicht bis unendlich - auf beiden Seiten des Vergleichs.
	//
	// Bei einer Korrektur ist der ersetzte Eintrag in dieser Transaktion
	// bereits storniert und faellt ueber status='active' heraus.
	var overlapping bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM hr_time_entries
			 WHERE employee_id=$1 AND status='active'
			   AND started_at < COALESCE($3::timestamptz, 'infinity'::timestamptz)
			   AND COALESCE(ended_at, 'infinity'::timestamptz) > $2
		)
	`, entry.EmployeeID, entry.StartedAt, entry.EndedAt).Scan(&overlapping); err != nil {
		return err
	}
	if overlapping {
		return errors.New("Zeitraum überschneidet sich mit einem bestehenden Zeiteintrag")
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO hr_time_entries (id, employee_id, work_date, started_at, ended_at, break_minutes, project_id, note, replaces_entry_id, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, entry.ID, entry.EmployeeID, entry.WorkDate, entry.StartedAt, entry.EndedAt, entry.BreakMinutes, entry.ProjectID, entry.Note, replaces, nullableUserID(actorUserID))
	return err
}

// lockTimeEntryTx laedt einen Eintrag mandantengeprueft und sperrt seine
// Zeile bis zum Ende der Transaktion.
func lockTimeEntryTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, companyID string) (*TimeEntry, error) {
	entry, err := scanTimeEntry(tx.QueryRow(ctx, `
		SELECT `+timeEntryColumns+`
		  FROM hr_time_entries t
		  JOIN hr_employees e ON e.id = t.employee_id
		 WHERE t.id=$1 AND e.company_id=$2
		   FOR UPDATE OF t
	`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Zeiteintrag nicht gefunden")
		}
		return nil, err
	}
	return entry, nil
}

// Stop beendet einen laufenden Eintrag. Der Uebergang ist nur einmal
// moeglich: ein beendeter Eintrag laesst sich nicht erneut beenden.
func (s *TimeEntryService) Stop(ctx context.Context, id uuid.UUID, endedAt time.Time, breakMinutes int, companyID string) (*TimeEntry, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Zeiteintrag-ID erforderlich")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	entry, err := lockTimeEntryTx(ctx, tx, id, companyID)
	if err != nil {
		return nil, err
	}
	if entry.Status == TimeEntryStatusStorniert {
		return nil, errors.New("Zeiteintrag ist storniert")
	}
	if entry.EndedAt != nil {
		return nil, errors.New("Zeiteintrag ist bereits beendet")
	}

	// Der VOLLSTAENDIGE Eintrag wird erneut geprueft, nicht nur die beiden
	// neuen Angaben: erst jetzt sind Ende und Pause gegen den Beginn
	// pruefbar.
	entry.EndedAt = &endedAt
	entry.BreakMinutes = breakMinutes
	if err := ValidateTimeEntry(*entry, time.Now()); err != nil {
		return nil, err
	}

	// Keine Ueberschneidungspruefung: ein laufender Eintrag galt bisher als
	// bis unendlich reichend, jeder spaetere Eintrag wurde deshalb beim
	// Anlegen abgelehnt. Das Beenden verkuerzt den Zeitraum nur.
	ct, err := tx.Exec(ctx, `
		UPDATE hr_time_entries SET ended_at=$2, break_minutes=$3
		 WHERE id=$1 AND ended_at IS NULL AND status='active'
	`, id, endedAt, breakMinutes)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Zeiteintrag ist bereits beendet")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// stornoTimeEntryTx storniert einen Eintrag innerhalb einer Transaktion des
// Aufrufers und liefert den Eintrag im Zustand VOR dem Storno. Der
// Uebergang ist nur einmal moeglich. Geschrieben werden ausschliesslich
// die vier Storno-Spalten; Zeiten, Pause, Projekt und Notiz bleiben stehen
// - das Original soll nachlesbar bleiben.
func stornoTimeEntryTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, reason, actorUserID, companyID string) (*TimeEntry, error) {
	entry, err := lockTimeEntryTx(ctx, tx, id, companyID)
	if err != nil {
		return nil, err
	}
	if entry.Status == TimeEntryStatusStorniert {
		return nil, errors.New("Zeiteintrag ist bereits storniert")
	}
	ct, err := tx.Exec(ctx, `
		UPDATE hr_time_entries SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3
		 WHERE id=$1 AND status='active'
	`, id, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Zeiteintrag ist bereits storniert")
	}
	return entry, nil
}

func validateStornoRequest(id uuid.UUID, reason, companyID string) (string, error) {
	if strings.TrimSpace(companyID) == "" {
		return "", errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return "", errors.New("Zeiteintrag-ID erforderlich")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", errors.New("Storno-Begründung erforderlich")
	}
	return reason, nil
}

// Storno storniert einen Eintrag ersatzlos (z. B. eine Fehlstempelung).
// Auch ein laufender Eintrag kann storniert werden.
func (s *TimeEntryService) Storno(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*TimeEntry, error) {
	reason, err := validateStornoRequest(id, reason, companyID)
	if err != nil {
		return nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := stornoTimeEntryTx(ctx, tx, id, reason, actorUserID, companyID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// Correct korrigiert einen Eintrag: der alte wird storniert, ein neuer mit
// den berichtigten Angaben angelegt und ueber replaces_entry_id mit dem
// alten verknuepft. Beides geschieht in EINER Transaktion - scheitert der
// Ersatzeintrag (z. B. an einer Ueberschneidung), bleibt der alte Eintrag
// unveraendert aktiv. Ein Storno ohne Ersatz waere sonst ein stiller
// Datenverlust.
//
// Der Mitarbeiter ist nicht korrigierbar: in.EmployeeID muss leer sein
// oder dem des alten Eintrags entsprechen. Eine Buchung auf den falschen
// Mitarbeiter wird storniert und fuer den richtigen neu erfasst.
//
// Geliefert wird der neue Eintrag.
func (s *TimeEntryService) Correct(ctx context.Context, id uuid.UUID, in TimeEntryInput, reason, actorUserID, companyID string) (*TimeEntry, error) {
	reason, err := validateStornoRequest(id, reason, companyID)
	if err != nil {
		return nil, err
	}
	replacement := newTimeEntry(in)
	if err := ValidateTimeEntry(replacement, time.Now()); err != nil {
		return nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	old, err := stornoTimeEntryTx(ctx, tx, id, reason, actorUserID, companyID)
	if err != nil {
		return nil, err
	}
	if in.EmployeeID != uuid.Nil && in.EmployeeID != old.EmployeeID {
		return nil, errors.New("Korrektur kann den Mitarbeiter nicht ändern")
	}
	replacement.EmployeeID = old.EmployeeID

	if err := insertTimeEntryTx(ctx, tx, replacement, &old.ID, actorUserID, companyID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, replacement.ID, companyID)
}

func (s *TimeEntryService) Get(ctx context.Context, id uuid.UUID, companyID string) (*TimeEntry, error) {
	entry, err := scanTimeEntry(s.pg.QueryRow(ctx, `
		SELECT `+timeEntryColumns+`
		  FROM hr_time_entries t
		  JOIN hr_employees e ON e.id = t.employee_id
		 WHERE t.id=$1 AND e.company_id=$2
	`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Zeiteintrag nicht gefunden")
		}
		return nil, err
	}
	return entry, nil
}
