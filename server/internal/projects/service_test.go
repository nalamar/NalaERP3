package projects

import (
	"context"
	"testing"
)

func TestCreateRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil)

	project, err := svc.Create(context.Background(), ProjectCreate{Name: "Test"}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if project != nil {
		t.Fatalf("expected nil project, got %#v", project)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestSetKostenstelleRejectsMissingID(t *testing.T) {
	svc := NewService(nil)

	project, err := svc.SetKostenstelle(context.Background(), "", nil, "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if project != nil {
		t.Fatalf("expected nil project, got %#v", project)
	}
	if err.Error() != "Projekt-ID erforderlich" {
		t.Fatalf("expected Projekt-ID erforderlich, got %q", err.Error())
	}
}

func TestCreateRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	project, err := svc.Create(context.Background(), ProjectCreate{}, "company-1")
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

	phase, err := svc.CreatePhase(context.Background(), "project-1", PhaseCreate{}, "company-1")
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

	_, err := svc.UpdatePhase(context.Background(), "project-1", "phase-1", PhaseUpdate{Nummer: &empty}, "company-1")
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

	_, err := svc.UpdatePhase(context.Background(), "project-1", "phase-1", PhaseUpdate{Name: &empty}, "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestCreateElevationRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	elevation, err := svc.CreateElevation(context.Background(), "project-1", "phase-1", ElevationCreate{}, "company-1")
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

	_, err := svc.UpdateElevation(context.Background(), "project-1", "phase-1", "elevation-1", ElevationUpdate{Nummer: &empty}, "company-1")
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

	_, err := svc.UpdateElevation(context.Background(), "project-1", "phase-1", "elevation-1", ElevationUpdate{Name: &empty}, "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestCreateSingleElevationRejectsMissingName(t *testing.T) {
	svc := NewService(nil)

	variant, err := svc.CreateSingleElevation(context.Background(), "project-1", "elevation-1", SingleElevationCreate{}, "company-1")
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

	_, err := svc.UpdateSingleElevation(context.Background(), "project-1", "elevation-1", "single-1", SingleElevationUpdate{Name: &empty}, "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestLinkVariantMaterialRejectsMissingParameters(t *testing.T) {
	svc := NewService(nil)

	err := svc.LinkVariantMaterial(context.Background(), "project-1", "single-1", "", "", "", "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Parameter" {
		t.Fatalf("expected Ungültige Parameter, got %q", err.Error())
	}
}

func TestLinkVariantMaterialRejectsInvalidKind(t *testing.T) {
	svc := NewService(nil)

	err := svc.LinkVariantMaterial(context.Background(), "project-1", "single-1", "invalid", "item-1", "", "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Typ" {
		t.Fatalf("expected Ungültiger Typ, got %q", err.Error())
	}
}
