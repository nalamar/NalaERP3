package quotes

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestComputeCalculationTotalsSeparatesCostCategoriesWithOwnSurcharges(t *testing.T) {
	c := &QuoteItemCalculation{
		MaterialCost:                 100,
		MaterialZuschlagPercent:      15,
		LohnStunden:                  2,
		LohnStundensatz:              40,
		LohnZuschlagPercent:          80,
		FremdleistungCost:            50,
		FremdleistungZuschlagPercent: 10,
	}
	computeCalculationTotals(c)

	if c.MaterialTotal != 115 {
		t.Fatalf("expected material_total 115 (100*1.15), got %v", c.MaterialTotal)
	}
	if c.LohnCost != 80 {
		t.Fatalf("expected lohn_cost 80 (2*40), got %v", c.LohnCost)
	}
	if c.LohnTotal != 144 {
		t.Fatalf("expected lohn_total 144 (80*1.80), got %v", c.LohnTotal)
	}
	if c.FremdleistungTotal != 55 {
		t.Fatalf("expected fremdleistung_total 55 (50*1.10), got %v", c.FremdleistungTotal)
	}
	if c.CalculatedUnitPrice != 314 {
		t.Fatalf("expected calculated_unit_price 314 (115+144+55), got %v", c.CalculatedUnitPrice)
	}
}

func TestComputeCalculationTotalsZeroInputsYieldZeroPrice(t *testing.T) {
	c := &QuoteItemCalculation{}
	computeCalculationTotals(c)
	if c.CalculatedUnitPrice != 0 {
		t.Fatalf("expected calculated_unit_price 0, got %v", c.CalculatedUnitPrice)
	}
}

func TestValidateCalculationInputRejectsNegativeMaterialCost(t *testing.T) {
	err := validateCalculationInput(QuoteItemCalculationInput{MaterialCost: -1})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Materialkosten dürfen nicht negativ sein" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestValidateCalculationInputRejectsNegativeLohnStundensatz(t *testing.T) {
	err := validateCalculationInput(QuoteItemCalculationInput{LohnStundensatz: -1})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Stundensatz darf nicht negativ sein" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestValidateCalculationInputAllowsAllZero(t *testing.T) {
	if err := validateCalculationInput(QuoteItemCalculationInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertQuoteItemCalculationRejectsInvalidInputBeforeDB(t *testing.T) {
	s := &Service{}

	_, err := s.UpsertQuoteItemCalculation(context.Background(), uuid.New(), uuid.New(), QuoteItemCalculationInput{FremdleistungCost: -5}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Fremdleistungskosten dürfen nicht negativ sein" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}
