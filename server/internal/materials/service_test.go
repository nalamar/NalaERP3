package materials

import (
	"context"
	"testing"
)

func TestCreateRejectsMissingRequiredFields(t *testing.T) {
	svc := NewService(nil, nil, "")

	got, err := svc.Create(context.Background(), MaterialCreate{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil material, got %#v", got)
	}
	if err.Error() != "Nummer und Bezeichnung sind erforderlich" {
		t.Fatalf("expected Nummer und Bezeichnung sind erforderlich, got %q", err.Error())
	}
}

func TestUpdateRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.Update(context.Background(), "", MaterialUpdate{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestUpdateRejectsEmptyNummer(t *testing.T) {
	svc := NewService(nil, nil, "")
	empty := ""

	_, err := svc.Update(context.Background(), "mat-1", MaterialUpdate{Nummer: &empty})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Nummer erforderlich" {
		t.Fatalf("expected Nummer erforderlich, got %q", err.Error())
	}
}

func TestUpdateRejectsEmptyBezeichnung(t *testing.T) {
	svc := NewService(nil, nil, "")
	empty := ""

	_, err := svc.Update(context.Background(), "mat-1", MaterialUpdate{Bezeichnung: &empty})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Bezeichnung erforderlich" {
		t.Fatalf("expected Bezeichnung erforderlich, got %q", err.Error())
	}
}

func TestUpdateRejectsEmptyEinheit(t *testing.T) {
	svc := NewService(nil, nil, "")
	empty := ""

	_, err := svc.Update(context.Background(), "mat-1", MaterialUpdate{Einheit: &empty})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Einheit erforderlich" {
		t.Fatalf("expected Einheit erforderlich, got %q", err.Error())
	}
}

func TestDeleteSoftRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	err := svc.DeleteSoft(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}
