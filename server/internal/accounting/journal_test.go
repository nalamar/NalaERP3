package accounting

import (
	"context"
	"testing"
	"time"
)

func TestJournalCreateRejectsMissingCompanyID(t *testing.T) {
	svc := NewJournalService(nil)

	entry, err := svc.Create(context.Background(), JournalEntryInput{
		Date: time.Now(),
		Lines: []JournalLineInput{
			{AccountCode: "1400", Debit: 100, Credit: 0},
			{AccountCode: "8400", Debit: 0, Credit: 100},
		},
	}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if entry != nil {
		t.Fatalf("expected nil entry, got %#v", entry)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected 'Mandant erforderlich', got %q", err.Error())
	}
}

func TestCreateRejectsEmptyLines(t *testing.T) {
	svc := NewJournalService(nil)

	entry, err := svc.Create(context.Background(), JournalEntryInput{
		Date:        time.Now(),
		Description: "Testbuchung",
	}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if entry != nil {
		t.Fatalf("expected nil entry, got %#v", entry)
	}
	if err.Error() != "keine Buchungszeilen" {
		t.Fatalf("expected 'keine Buchungszeilen', got %q", err.Error())
	}
}

func TestCreateRejectsMissingAccountCode(t *testing.T) {
	svc := NewJournalService(nil)

	entry, err := svc.Create(context.Background(), JournalEntryInput{
		Date: time.Now(),
		Lines: []JournalLineInput{
			{AccountCode: "", Debit: 100, Credit: 0},
			{AccountCode: "1600", Debit: 0, Credit: 100},
		},
	}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if entry != nil {
		t.Fatalf("expected nil entry, got %#v", entry)
	}
	if err.Error() != "account_code fehlt" {
		t.Fatalf("expected 'account_code fehlt', got %q", err.Error())
	}
}

func TestCreateRejectsUnbalancedLines(t *testing.T) {
	svc := NewJournalService(nil)

	entry, err := svc.Create(context.Background(), JournalEntryInput{
		Date: time.Now(),
		Lines: []JournalLineInput{
			{AccountCode: "1400", Debit: 119, Credit: 0},
			{AccountCode: "8400", Debit: 0, Credit: 100},
		},
	}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if entry != nil {
		t.Fatalf("expected nil entry, got %#v", entry)
	}
	if err.Error() != "Soll/Haben nicht ausgeglichen" {
		t.Fatalf("expected 'Soll/Haben nicht ausgeglichen', got %q", err.Error())
	}
}
