package noticemail

import (
	"context"
	"testing"
)

// Vorpruefungen ohne Datenbank (nil-Pool, Backlog F.14.4c).
func TestPrefsValidateBeforeTouchingDatabase(t *testing.T) {
	p := NewPrefs(nil)
	ctx := context.Background()
	if _, err := p.OptedOut(ctx, " "); err == nil || err.Error() != "Benutzer erforderlich" {
		t.Fatalf("OptedOut: %v", err)
	}
	if err := p.SetOptedOut(ctx, "", true); err == nil || err.Error() != "Benutzer erforderlich" {
		t.Fatalf("SetOptedOut: %v", err)
	}
	if _, err := p.ListLog(ctx, " ", 10); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ListLog: %v", err)
	}
}
