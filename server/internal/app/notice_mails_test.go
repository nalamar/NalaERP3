package app

import (
	"testing"

	"nalaerp3/internal/config"
	"nalaerp3/internal/mail"
	"nalaerp3/internal/noticemail"
)

// Verdrahtung der Hinweis-Mails ohne Datenbank (Backlog F.14.4b).
func TestNewNoticeMailScheduler(t *testing.T) {
	if s := newNoticeMailScheduler(nil, &config.Config{NoticeMailsDisabled: true}); s != nil {
		t.Fatal("abgeschaltet, aber Zeitsteuerung gebaut")
	}
	s := newNoticeMailScheduler(nil, &config.Config{MailRedirectTo: "dev@example.test"})
	if s == nil {
		t.Fatal("keine Zeitsteuerung")
	}
	r, ok := s.Runner.(*noticemail.Runner)
	if !ok || r.Notices == nil || r.Settings == nil || r.Permissions == nil || r.Employees == nil {
		t.Fatalf("Runner: %+v", s.Runner)
	}
	// Die Umleitung der Entwicklung gilt auch fuer die Hinweis-Mails.
	if tr, ok := r.Transport.(*mail.Sender); !ok || tr.RedirectTo != "dev@example.test" {
		t.Fatalf("Versand: %+v", r.Transport)
	}
}
