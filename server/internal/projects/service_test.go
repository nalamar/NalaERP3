package projects

import (
	"context"
	"testing"
)

func TestCreateRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	project, err := svc.Create(context.Background(), ProjectCreate{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if project != nil {
		t.Fatalf("expected nil project, got %#v", project)
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestCreatePhaseRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	phase, err := svc.CreatePhase(context.Background(), "project-1", PhaseCreate{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if phase != nil {
		t.Fatalf("expected nil phase, got %#v", phase)
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestUpdatePhaseRejectsEmptyNummer(t *testing.T) {
	svc := NewService(nil)
	empty := ""

	_, err := svc.UpdatePhase(context.Background(), "phase-1", PhaseUpdate{Nummer: &empty})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Nummer erforderlich" {
		t.Fatalf("expected Nummer erforderlich, got %q", err.Error())
	}
}

func TestUpdatePhaseRejectsEmptyName(t *testing.T) {
	svc := NewService(nil)
	empty := ""

	_, err := svc.UpdatePhase(context.Background(), "phase-1", PhaseUpdate{Name: &empty})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestCreateElevationRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	elevation, err := svc.CreateElevation(context.Background(), "phase-1", ElevationCreate{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if elevation != nil {
		t.Fatalf("expected nil elevation, got %#v", elevation)
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestUpdateElevationRejectsEmptyNummer(t *testing.T) {
	svc := NewService(nil)
	empty := ""

	_, err := svc.UpdateElevation(context.Background(), "elevation-1", ElevationUpdate{Nummer: &empty})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Nummer erforderlich" {
		t.Fatalf("expected Nummer erforderlich, got %q", err.Error())
	}
}

func TestUpdateElevationRejectsEmptyName(t *testing.T) {
	svc := NewService(nil)
	empty := ""

	_, err := svc.UpdateElevation(context.Background(), "elevation-1", ElevationUpdate{Name: &empty})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestCreateSingleElevationRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	variant, err := svc.CreateSingleElevation(context.Background(), "elevation-1", SingleElevationCreate{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if variant != nil {
		t.Fatalf("expected nil single elevation, got %#v", variant)
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestUpdateSingleElevationRejectsEmptyName(t *testing.T) {
	svc := NewService(nil)
	empty := ""

	_, err := svc.UpdateSingleElevation(context.Background(), "single-1", SingleElevationUpdate{Name: &empty})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestLinkVariantMaterialRejectsMissingParameters(t *testing.T) {
	svc := NewService(nil)

	err := svc.LinkVariantMaterial(context.Background(), "", "", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Parameter" {
		t.Fatalf("expected Ungültige Parameter, got %q", err.Error())
	}
}

func TestLinkVariantMaterialRejectsInvalidKind(t *testing.T) {
	svc := NewService(nil)

	err := svc.LinkVariantMaterial(context.Background(), "invalid", "item-1", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Typ" {
		t.Fatalf("expected Ungültiger Typ, got %q", err.Error())
	}
}
