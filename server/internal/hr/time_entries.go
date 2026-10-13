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
// Eintraege sind unveraenderlich. Dieser Service kennt genau einen
// UPDATE auf die Zeiten eines Eintrags: das einmalige Beenden eines
// laufenden Eintrags in Stop. Alles andere ist INSERT. Es gibt keinen
// DELETE.
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
	entry := TimeEntry{
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
	if err := ValidateTimeEntry(entry, time.Now()); err != nil {
		return nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Die Zeile des Mitarbeiters wird gesperrt. Das prueft die
	// Mandantenzugehoerigkeit UND serialisiert alle Buchungen fuer
	// denselben Mitarbeiter: ohne die Sperre koennten zwei gleichzeitige
	// Buchungen die Ueberschneidungspruefung unten beide passieren.
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM hr_employees WHERE id=$1 AND company_id=$2 FOR UPDATE`, in.EmployeeID, companyID).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Mitarbeiter nicht gefunden")
		}
		return nil, err
	}

	if in.ProjectID != nil {
		var projectOwned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND company_id=$2)`, *in.ProjectID, companyID).Scan(&projectOwned); err != nil {
			return nil, err
		}
		if !projectOwned {
			return nil, errors.New("Projekt nicht gefunden")
		}
	}

	if in.EndedAt == nil {
		var running bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_time_entries WHERE employee_id=$1 AND status='active' AND ended_at IS NULL)`, in.EmployeeID).Scan(&running); err != nil {
			return nil, err
		}
		if running {
			return nil, errors.New("Es läuft bereits ein Zeiteintrag")
		}
	}

	// Ueberschneidung mit einem anderen aktiven Eintrag. Zeitraeume sind
	// halboffen [Beginn, Ende): ein Eintrag, der genau dort beginnt, wo
	// ein anderer endet, ueberschneidet sich nicht. Ein laufender Eintrag
	// reicht bis unendlich - auf beiden Seiten des Vergleichs.
	var overlapping bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM hr_time_entries
			 WHERE employee_id=$1 AND status='active'
			   AND started_at < COALESCE($3::timestamptz, 'infinity'::timestamptz)
			   AND COALESCE(ended_at, 'infinity'::timestamptz) > $2
		)
	`, in.EmployeeID, in.StartedAt, in.EndedAt).Scan(&overlapping); err != nil {
		return nil, err
	}
	if overlapping {
		return nil, errors.New("Zeitraum überschneidet sich mit einem bestehenden Zeiteintrag")
	}

	var createdBy *string
	if strings.TrimSpace(actorUserID) != "" {
		createdBy = &actorUserID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO hr_time_entries (id, employee_id, work_date, started_at, ended_at, break_minutes, project_id, note, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, entry.ID, entry.EmployeeID, entry.WorkDate, entry.StartedAt, entry.EndedAt, entry.BreakMinutes, entry.ProjectID, entry.Note, createdBy); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, entry.ID, companyID)
}

// Stop beendet einen laufenden Eintrag. Das ist der einzige Uebergang, der
// die Zeiten eines bestehenden Eintrags beruehrt, und er ist nur einmal
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
	// bis unendlich reichend, Create hat deshalb jeden spaeteren Eintrag
	// abgelehnt. Das Beenden verkuerzt den Zeitraum nur.
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
