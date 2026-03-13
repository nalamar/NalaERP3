package contacts

import (
	"context"
	"testing"
)

func TestCreateRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	got, err := svc.Create(context.Background(), ContactCreate{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil contact, got %#v", got)
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestCreateRejectsInvalidType(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.Create(context.Background(), ContactCreate{
		Name: "Test GmbH",
		Typ:  "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Typ" {
		t.Fatalf("expected Ungültiger Typ, got %q", err.Error())
	}
}

func TestCreateRejectsInvalidRole(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.Create(context.Background(), ContactCreate{
		Name:  "Test GmbH",
		Typ:   "org",
		Rolle: "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Rolle" {
		t.Fatalf("expected Ungültige Rolle, got %q", err.Error())
	}
}

func TestCreateRejectsInvalidStatus(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.Create(context.Background(), ContactCreate{
		Name:   "Test GmbH",
		Typ:    "org",
		Rolle:  "customer",
		Status: "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Status" {
		t.Fatalf("expected Ungültiger Status, got %q", err.Error())
	}
}

func TestUpdateRejectsInvalidRole(t *testing.T) {
	svc := NewService(nil)
	role := "invalid"

	_, err := svc.Update(context.Background(), "contact-1", ContactUpdate{Rolle: &role})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Rolle" {
		t.Fatalf("expected Ungültige Rolle, got %q", err.Error())
	}
}

func TestUpdateRejectsInvalidStatus(t *testing.T) {
	svc := NewService(nil)
	status := "invalid"

	_, err := svc.Update(context.Background(), "contact-1", ContactUpdate{Status: &status})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Status" {
		t.Fatalf("expected Ungültiger Status, got %q", err.Error())
	}
}

func TestNormalizeStatusAndActiveMapsBlockedToInactiveActiveFalse(t *testing.T) {
	status := "blocked"

	gotStatus, gotAktiv := normalizeStatusAndActive(&status, nil)
	if gotStatus != "blocked" {
		t.Fatalf("expected blocked, got %q", gotStatus)
	}
	if gotAktiv {
		t.Fatal("expected aktiv=false for blocked status")
	}
}

func TestNormalizeStatusAndActiveMapsInactiveFlagToInactiveStatus(t *testing.T) {
	aktiv := false

	gotStatus, gotAktiv := normalizeStatusAndActive(nil, &aktiv)
	if gotStatus != "inactive" {
		t.Fatalf("expected inactive, got %q", gotStatus)
	}
	if gotAktiv {
		t.Fatal("expected aktiv=false")
	}
}

func TestUpdateRejectsInvalidAddressKind(t *testing.T) {
	svc := NewService(nil)
	kind := "invalid"

	_, err := svc.UpdateAddress(context.Background(), "contact-1", "address-1", AddressUpdate{Art: &kind})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Adressart" {
		t.Fatalf("expected Ungültige Adressart, got %q", err.Error())
	}
}

func TestCreatePersonRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreatePerson(context.Background(), "contact-1", PersonCreate{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestCreatePersonRejectsInvalidRole(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreatePerson(context.Background(), "contact-1", PersonCreate{
		Vorname: "Max",
		Rolle:   "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Ansprechpartnerrolle" {
		t.Fatalf("expected Ungültige Ansprechpartnerrolle, got %q", err.Error())
	}
}

func TestCreatePersonRejectsInvalidChannel(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreatePerson(context.Background(), "contact-1", PersonCreate{
		Vorname:          "Max",
		BevorzugterKanal: "fax",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Kommunikationskanal" {
		t.Fatalf("expected Ungültiger Kommunikationskanal, got %q", err.Error())
	}
}
