package app

import (
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/assets"
	"nalaerp3/internal/auth"
	"nalaerp3/internal/config"
	"nalaerp3/internal/fleet"
	"nalaerp3/internal/hr"
	"nalaerp3/internal/mail"
	"nalaerp3/internal/noticemail"
	"nalaerp3/internal/notices"
	"nalaerp3/internal/secretbox"
)

// newNoticeMailScheduler verdrahtet den Versand der Hinweis-Mails (Backlog
// F.14.4b) mit denselben Diensten wie die API: dieselben Hinweise wie
// GET /notices, derselbe Mail-Zugang und - in der Entwicklung - dieselbe
// Umleitung MAIL_REDIRECT_TO. nil, wenn abgeschaltet.
func newNoticeMailScheduler(pg *pgxpool.Pool, cfg *config.Config) *noticemail.Scheduler {
	if cfg.NoticeMailsDisabled {
		log.Printf("[Hinweis-Mails] abgeschaltet (NOTICE_MAILS_DISABLED)")
		return nil
	}
	box, err := secretbox.FromEnvValue(cfg.SettingsEncryptionKey)
	if err != nil {
		box = nil // wie in der API: dann gilt der Zugang als nicht eingerichtet
	}
	return &noticemail.Scheduler{Runner: &noticemail.Runner{
		PG:          pg,
		Notices:     notices.NewDefaultService(hr.NewQualificationService(pg), assets.NewService(pg), fleet.NewScheduleService(pg)),
		Settings:    mail.NewSettingsService(pg, box),
		Transport:   &mail.Sender{RedirectTo: cfg.MailRedirectTo},
		Permissions: auth.NewRepository(pg),
		Employees:   hr.NewTimeEntryService(pg),
	}}
}
