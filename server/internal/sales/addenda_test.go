package sales

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestValidateAddendumStatusTransitionAllowsEntwurfToBeantragt(t *testing.T) {
	if err := validateAddendumStatusTransition("entwurf", "beantragt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAddendumStatusTransitionAllowsBeantragtToDecision(t *testing.T) {
	if err := validateAddendumStatusTransition("beantragt", "angenommen"); err != nil {
		t.Fatalf("unexpected error for angenommen: %v", err)
	}
	if err := validateAddendumStatusTransition("beantragt", "abgelehnt"); err != nil {
		t.Fatalf("unexpected error for abgelehnt: %v", err)
	}
}

func TestValidateAddendumStatusTransitionRejectsTerminalStatuses(t *testing.T) {
	for _, current := range []string{"angenommen", "abgelehnt"} {
		err := validateAddendumStatusTransition(current, "beantragt")
		if err == nil {
			t.Fatalf("expected validation error for terminal status %q, got nil", current)
		}
		if err.Error() != "entschiedene Nachträge können nicht erneut umgestellt werden" {
			t.Fatalf("unexpected error message for %q: %q", current, err.Error())
		}
	}
}

func TestValidateAddendumStatusTransitionRejectsSkippingBeantragt(t *testing.T) {
	err := validateAddendumStatusTransition("entwurf", "angenommen")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Nachtragsstatus darf nicht in den gewünschten Status wechseln" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestCreateAddendumRejectsMissingBegruendung(t *testing.T) {
	s := &Service{}

	_, err := s.CreateAddendum(context.Background(), uuid.New(), SalesOrderAddendumCreate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Begründung erforderlich" {
		t.Fatalf("expected Begründung erforderlich, got %q", err.Error())
	}
}

func TestCreateAddendumItemRejectsMissingDescription(t *testing.T) {
	s := &Service{}

	_, err := s.CreateAddendumItem(context.Background(), uuid.New(), uuid.New(), SalesOrderAddendumItemInput{Qty: 1, Unit: "Stk"}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Positionsbeschreibung erforderlich" {
		t.Fatalf("expected Positionsbeschreibung erforderlich, got %q", err.Error())
	}
}

func TestDecideAddendumRejectsMissingAblehnungsgrundWhenRejecting(t *testing.T) {
	s := &Service{}

	_, err := s.DecideAddendum(context.Background(), uuid.New(), uuid.New(), false, SalesOrderAddendumRejection{}, "default", "user-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ablehnungsgrund erforderlich" {
		t.Fatalf("expected Ablehnungsgrund erforderlich, got %q", err.Error())
	}
}
