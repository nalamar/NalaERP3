package quotes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

func TestQuoteImportParseResultStoresItemsAndUpdatesStatus(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	contactID := uuid.NewString()
	projectID := uuid.NewString()

	_, err := env.PG.Exec(ctx, `
		INSERT INTO contacts (id, typ, rolle, status, name, email, telefon, waehrung)
		VALUES ($1,'org','customer','active',$2,$3,$4,'EUR')
	`, contactID, "GAEB Import Kunde GmbH", "gaeb-import@example.com", "+49 211 555555")
	if err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	_, err = env.PG.Exec(ctx, `
		INSERT INTO projects (id, name, kunde_id, status)
		VALUES ($1,$2,$3,'angebot')
	`, projectID, "GAEB Import Projekt", contactID)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}

	svc := NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)

	created, err := svc.CreateGAEBImport(ctx, QuoteImportCreateInput{
		ProjectID: projectID,
		ContactID: contactID,
	}, strings.NewReader("dummy-gaeb"), "lv-test.x83")
	if err != nil {
		t.Fatalf("create gaeb import: %v", err)
	}
	if created.Status != "uploaded" {
		t.Fatalf("expected uploaded status, got %q", created.Status)
	}

	parsed, err := svc.SaveImportParseResult(ctx, created.ID, "parser-v1", "x83", []QuoteImportItemInput{
		{
			PositionNo:  "01.001",
			OutlineNo:   "01",
			Description: "Fensterelement Aluminium",
			Qty:         2,
			Unit:        "Stk",
			ParserHint:  "lv-position",
			SortOrder:   1,
		},
		{
			PositionNo:  "01.002",
			OutlineNo:   "01",
			Description: "Montage vor Ort",
			Qty:         6,
			Unit:        "Std",
			IsOptional:  true,
			ParserHint:  "optionale-position",
			SortOrder:   2,
		},
	})
	if err != nil {
		t.Fatalf("save parse result: %v", err)
	}
	if parsed.Status != "parsed" {
		t.Fatalf("expected parsed status, got %q", parsed.Status)
	}
	if parsed.ParserVersion != "parser-v1" {
		t.Fatalf("expected parser version parser-v1, got %q", parsed.ParserVersion)
	}
	if parsed.DetectedFormat != "x83" {
		t.Fatalf("expected detected format x83, got %q", parsed.DetectedFormat)
	}
	if parsed.ErrorMessage != "" {
		t.Fatalf("expected empty error_message after parsed import, got %q", parsed.ErrorMessage)
	}
	if parsed.ItemCount != 2 {
		t.Fatalf("expected item_count 2 after parse result, got %d", parsed.ItemCount)
	}

	items, err := svc.ListImportItems(ctx, created.ID)
	if err != nil {
		t.Fatalf("list import items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 import items, got %d", len(items))
	}
	if items[0].PositionNo != "01.001" || items[0].ReviewStatus != "pending" || items[0].SortOrder != 1 {
		t.Fatalf("unexpected first import item: %+v", items[0])
	}
	if items[1].PositionNo != "01.002" || !items[1].IsOptional || items[1].ReviewStatus != "pending" || items[1].SortOrder != 2 {
		t.Fatalf("unexpected second import item: %+v", items[1])
	}

	failed, err := svc.MarkImportFailed(ctx, created.ID, "parser-v1", "x83", "GAEB-Struktur konnte nicht vollständig gelesen werden")
	if err != nil {
		t.Fatalf("mark import failed: %v", err)
	}
	if failed.Status != "failed" {
		t.Fatalf("expected failed status, got %q", failed.Status)
	}
	if failed.ErrorMessage == "" {
		t.Fatal("expected error_message on failed import")
	}
	if failed.ItemCount != 0 {
		t.Fatalf("expected item_count 0 after failed transition, got %d", failed.ItemCount)
	}

	itemsAfterFailed, err := svc.ListImportItems(ctx, created.ID)
	if err != nil {
		t.Fatalf("list import items after failed transition: %v", err)
	}
	if len(itemsAfterFailed) != 0 {
		t.Fatalf("expected no import items after failed transition, got %d", len(itemsAfterFailed))
	}
}
