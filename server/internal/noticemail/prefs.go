package noticemail

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Abbestellung je Benutzer und Versandprotokoll lesen (Backlog F.14.4c).

type Prefs struct{ pg *pgxpool.Pool }

func NewPrefs(pg *pgxpool.Pool) *Prefs { return &Prefs{pg: pg} }

// OptedOut: hat der Benutzer die Hinweis-Mails abbestellt?
func (p *Prefs) OptedOut(ctx context.Context, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, errors.New("Benutzer erforderlich")
	}
	var out bool
	err := p.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notice_mail_optouts WHERE user_id=$1)`, userID).Scan(&out)
	return out, err
}

// SetOptedOut bestellt die Hinweis-Mails ab (true) oder wieder an (false).
// Wiederholen ist unschaedlich.
func (p *Prefs) SetOptedOut(ctx context.Context, userID string, optOut bool) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("Benutzer erforderlich")
	}
	var err error
	if optOut {
		_, err = p.pg.Exec(ctx, `INSERT INTO notice_mail_optouts (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID)
	} else {
		_, err = p.pg.Exec(ctx, `DELETE FROM notice_mail_optouts WHERE user_id=$1`, userID)
	}
	return err
}

// LogEntry ist ein Eintrag des Versandprotokolls - ohne Inhalt und ohne
// Adresse. Benutzer ist der Anzeigename (sonst der Benutzername); leer,
// wenn der Benutzer entfernt wurde.
type LogEntry struct {
	VersandTag  time.Time  `json:"versand_tag"`
	Benutzer    string     `json:"benutzer"`
	Ergebnis    string     `json:"ergebnis"`
	FehlerStufe string     `json:"fehler_stufe"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

const (
	DefaultLogLimit = 100
	MaxLogLimit     = 500
)

// ListLog liefert das Protokoll des Mandanten, das juengste zuerst.
// limit <= 0: DefaultLogLimit; hoechstens MaxLogLimit.
func (p *Prefs) ListLog(ctx context.Context, companyID string, limit int) ([]LogEntry, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if limit <= 0 {
		limit = DefaultLogLimit
	}
	if limit > MaxLogLimit {
		limit = MaxLogLimit
	}
	rows, err := p.pg.Query(ctx, `
		SELECT l.versand_tag, COALESCE(NULLIF(btrim(u.display_name), ''), u.username, ''), l.ergebnis, l.fehler_stufe, l.created_at, l.finished_at
		  FROM notice_mail_log l
		  LEFT JOIN users u ON u.id = l.user_id
		 WHERE l.company_id=$1
		 ORDER BY l.created_at DESC, l.id
		 LIMIT $2`, companyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LogEntry, 0)
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.VersandTag, &e.Benutzer, &e.Ergebnis, &e.FehlerStufe, &e.CreatedAt, &e.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
