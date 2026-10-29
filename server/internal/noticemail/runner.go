package noticemail

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/mail"
	"nalaerp3/internal/notices"
)

// Ergebnisse im Versandprotokoll.
const (
	ErgebnisVersand  = "versand" // angelegt, Versand laeuft (oder brach ab)
	ErgebnisGesendet = "gesendet"
	ErgebnisFehler   = "fehler"
)

// Stufe im Protokoll, wenn der Fehler kein Versandfehler des Mail-Servers
// ist (z. B. ungueltige Adresse des Benutzers).
const StufeEingabe = "eingabe"

// Wie apihttp.adminSuperuserPermission: der Superuser hat jedes Recht.
const superuserPermission = "admin.superuser"

// Kleine Schnittstellen - erfuellt von notices.Service,
// mail.SettingsService, mail.Sender, auth.Repository und
// hr.TimeEntryService; Tests setzen Attrappen ein.
type NoticeSource interface {
	For(ctx context.Context, r notices.Recipient, today time.Time) ([]notices.Notice, error)
}

type ConfigSource interface {
	Config(ctx context.Context, companyID string) (*mail.SMTPConfig, error)
}

type Transport interface {
	Send(ctx context.Context, cfg mail.SMTPConfig, msg mail.Message) error
}

type PermissionLister interface {
	ListPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error)
}

type EmployeeResolver interface {
	EmployeeIDForUser(ctx context.Context, userID, companyID string) (uuid.UUID, bool, error)
}

type Runner struct {
	PG          *pgxpool.Pool
	Notices     NoticeSource
	Settings    ConfigSource
	Transport   Transport
	Permissions PermissionLister
	Employees   EmployeeResolver
}

// Result zaehlt einen Lauf.
type Result struct {
	Gesendet int `json:"gesendet"`
	Fehler   int `json:"fehler"`
}

// RunDay verschickt die Sammelmails fuer den Versandtag day an alle
// Mandanten mit eingeschaltetem Versand. Benutzer, die abbestellt haben,
// fuer die heute schon ein Protokolleintrag besteht oder die nichts
// Faelliges haben, bekommen nichts. Ein Fehler beim Versand an einen
// Benutzer bricht den Lauf nicht ab; er steht im Protokoll.
func (r *Runner) RunDay(ctx context.Context, day time.Time) (Result, error) {
	var res Result
	rows, err := r.PG.Query(ctx, `SELECT company_id FROM company_mail_settings WHERE aktiv ORDER BY company_id`)
	if err != nil {
		return res, err
	}
	companies := make([]string, 0)
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return res, err
		}
		companies = append(companies, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}
	for _, companyID := range companies {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		cfg, err := r.Settings.Config(ctx, companyID)
		if err != nil || !cfg.Aktiv {
			// Nicht (mehr) vollstaendig eingerichtet oder zwischenzeitlich
			// ausgeschaltet: dieser Mandant faellt heute aus.
			continue
		}
		if err := r.runCompany(ctx, *cfg, companyID, day, &res); err != nil {
			return res, err
		}
	}
	return res, nil
}

type recipient struct{ id, email string }

func (r *Runner) runCompany(ctx context.Context, cfg mail.SMTPConfig, companyID string, day time.Time, res *Result) error {
	rows, err := r.PG.Query(ctx, `
		SELECT u.id, u.email FROM users u
		 WHERE u.company_id=$1 AND u.is_active AND NOT u.is_locked AND btrim(u.email) <> ''
		   AND NOT EXISTS (SELECT 1 FROM notice_mail_optouts o WHERE o.user_id=u.id)
		   AND NOT EXISTS (SELECT 1 FROM notice_mail_log l WHERE l.company_id=$1 AND l.user_id=u.id AND l.versand_tag=$2)
		 ORDER BY u.id`, companyID, day)
	if err != nil {
		return err
	}
	users := make([]recipient, 0)
	for rows.Next() {
		var u recipient
		if err := rows.Scan(&u.id, &u.email); err != nil {
			rows.Close()
			return err
		}
		users = append(users, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, u := range users {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.runUser(ctx, cfg, companyID, u, day, res); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) runUser(ctx context.Context, cfg mail.SMTPConfig, companyID string, u recipient, day time.Time, res *Result) error {
	perms, err := r.Permissions.ListPermissionCodesByUserID(ctx, u.id)
	if err != nil {
		return err
	}
	rc := notices.Recipient{
		UserID:    u.id,
		CompanyID: companyID,
		Can: func(p string) bool {
			return slices.Contains(perms, p) || slices.Contains(perms, superuserPermission)
		},
	}
	own, linked, err := r.Employees.EmployeeIDForUser(ctx, u.id, companyID)
	if err != nil {
		return err
	}
	if linked {
		rc.EmployeeID = &own
	}
	ns, err := r.Notices.For(ctx, rc, day)
	if err != nil {
		return err
	}
	subject, text, ok := BuildDigest(ns)
	if !ok {
		return nil // nichts faellig: keine Mail, kein Protokoll
	}

	// Erst den Eintrag anlegen - besteht er schon (zweiter Prozess), wird
	// nicht versandt.
	logID := uuid.New()
	ct, err := r.PG.Exec(ctx, `
		INSERT INTO notice_mail_log (id, company_id, user_id, versand_tag, ergebnis)
		VALUES ($1,$2,$3,$4,'versand')
		ON CONFLICT (company_id, user_id, versand_tag) DO NOTHING`, logID, companyID, u.id, day)
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return nil
	}

	sendErr := r.Transport.Send(ctx, cfg, mail.Message{To: []string{u.email}, Subject: subject, Text: text})
	ergebnis, stufe := ErgebnisGesendet, ""
	if sendErr != nil {
		ergebnis, stufe = ErgebnisFehler, StufeEingabe
		var se *mail.SendError
		if errors.As(sendErr, &se) && se.Stage != "" {
			stufe = se.Stage
		}
		res.Fehler++
	} else {
		res.Gesendet++
	}
	// Eigener Kontext: auch nach Abbruch des Laufs soll das Ergebnis im
	// Protokoll stehen.
	upd, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	_, err = r.PG.Exec(upd, `UPDATE notice_mail_log SET ergebnis=$2, fehler_stufe=$3, finished_at=now() WHERE id=$1`, logID, ergebnis, stufe)
	return err
}
