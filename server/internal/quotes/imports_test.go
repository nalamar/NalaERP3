package quotes

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

type fakeGAEBImportParser struct {
	result    GAEBImportParseResult
	err       error
	filenames []string
	sources   []string
}

func (p *fakeGAEBImportParser) ParseGAEB(_ context.Context, source io.Reader, filename string) (GAEBImportParseResult, error) {
	bytes, err := io.ReadAll(source)
	if err != nil {
		return GAEBImportParseResult{}, err
	}
	p.filenames = append(p.filenames, filename)
	p.sources = append(p.sources, string(bytes))
	if p.err != nil {
		return GAEBImportParseResult{}, p.err
	}
	return p.result, nil
}

const testGAEBImportCompanyID = "default"

func createUploadedGAEBImportForProcessingTest(t *testing.T, ctx context.Context, parser GAEBImportParser) (*Service, *QuoteImport) {
	t.Helper()
	env := testutil.SetupIntegrationEnv(t)
	contactID := uuid.NewString()
	projectID := uuid.NewString()
	if _, err := env.PG.Exec(ctx, "INSERT INTO contacts (id, typ, rolle, status, name, email, phone, waehrung) VALUES ($1,'org','customer','active',$2,$3,$4,'EUR')", contactID, "GAEB Verarbeitung Kunde GmbH", "gaeb-processing@example.com", "+49 211 555555"); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := env.PG.Exec(ctx, "INSERT INTO projects (id, nummer, name, kunde_id, status, company_id) VALUES ($1,$2,$3,$4,'angebot',$5)", projectID, "PRJ-GAEB-PROCESSING-0001", "GAEB Verarbeitung Projekt", contactID, testGAEBImportCompanyID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	svc := NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB).WithGAEBImportParser(parser)
	created, err := svc.CreateGAEBImport(ctx, QuoteImportCreateInput{
		ProjectID: projectID,
		ContactID: contactID,
	}, strings.NewReader("gaeb-source-payload"), "verarbeitung.x83", testGAEBImportCompanyID)
	if err != nil {
		t.Fatalf("create gaeb import: %v", err)
	}
	return svc, created
}

func TestProcessGAEBImportStoresParserResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	parser := &fakeGAEBImportParser{result: GAEBImportParseResult{
		ParserVersion:  "fake-parser-v1",
		DetectedFormat: "x83",
		Items: []QuoteImportItemInput{
			{PositionNo: "01.001", Description: "Aluminiumfenster", Qty: 2, Unit: "Stk", SortOrder: 1},
			{PositionNo: "01.002", Description: "Montage", Qty: 4, Unit: "Std", SortOrder: 2},
		},
	}}
	svc, created := createUploadedGAEBImportForProcessingTest(t, ctx, parser)

	processed, err := svc.ProcessGAEBImport(ctx, created.ID, testGAEBImportCompanyID)
	if err != nil {
		t.Fatalf("process gaeb import: %v", err)
	}
	if processed.Status != "parsed" || processed.ParserVersion != "fake-parser-v1" || processed.DetectedFormat != "x83" || processed.ItemCount != 2 {
		t.Fatalf("unexpected processed import: %+v", processed)
	}
	if len(parser.filenames) != 1 || parser.filenames[0] != "verarbeitung.x83" || len(parser.sources) != 1 || parser.sources[0] != "gaeb-source-payload" {
		t.Fatalf("unexpected parser calls: %+v", parser)
	}
}

func TestProcessGAEBImportMarksParserFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	parser := &fakeGAEBImportParser{err: io.ErrUnexpectedEOF}
	svc, created := createUploadedGAEBImportForProcessingTest(t, ctx, parser)

	processed, err := svc.ProcessGAEBImport(ctx, created.ID, testGAEBImportCompanyID)
	if err != nil {
		t.Fatalf("process failing gaeb import: %v", err)
	}
	if processed.Status != "failed" || processed.ParserVersion != "adapter-v1" || processed.DetectedFormat != "x83" || !strings.Contains(processed.ErrorMessage, io.ErrUnexpectedEOF.Error()) || processed.ItemCount != 0 {
		t.Fatalf("unexpected failed import: %+v", processed)
	}
	if len(parser.filenames) != 1 || len(parser.sources) != 1 {
		t.Fatalf("expected one parser call, got %+v", parser)
	}
}

func TestProcessGAEBImportRequiresConfiguredParserWithoutMutation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	svc, created := createUploadedGAEBImportForProcessingTest(t, ctx, nil)

	processed, err := svc.ProcessGAEBImport(ctx, created.ID, testGAEBImportCompanyID)
	if err == nil || !strings.Contains(err.Error(), "GAEB-Parser nicht konfiguriert") {
		t.Fatalf("expected missing parser error, got import=%+v err=%v", processed, err)
	}
	fetched, err := svc.GetImport(ctx, created.ID, testGAEBImportCompanyID)
	if err != nil {
		t.Fatalf("get import after guard: %v", err)
	}
	if fetched.Status != "uploaded" || fetched.ItemCount != 0 {
		t.Fatalf("guard mutated import: %+v", fetched)
	}
}

func TestQuoteImportParseResultStoresItemsAndUpdatesStatus(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	contactID := uuid.NewString()
	projectID := uuid.NewString()

	_, err := env.PG.Exec(ctx, `
		INSERT INTO contacts (id, typ, rolle, status, name, email, phone, waehrung)
		VALUES ($1,'org','customer','active',$2,$3,$4,'EUR')
	`, contactID, "GAEB Import Kunde GmbH", "gaeb-import@example.com", "+49 211 555555")
	if err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	_, err = env.PG.Exec(ctx, `
		INSERT INTO projects (id, nummer, name, kunde_id, status, company_id)
		VALUES ($1,$2,$3,$4,'angebot',$5)
	`, projectID, "PRJ-GAEB-IMPORT-0001", "GAEB Import Projekt", contactID, testGAEBImportCompanyID)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}

	svc := NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)

	created, err := svc.CreateGAEBImport(ctx, QuoteImportCreateInput{
		ProjectID: projectID,
		ContactID: contactID,
	}, strings.NewReader("dummy-gaeb"), "lv-test.x83", testGAEBImportCompanyID)
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
	}, testGAEBImportCompanyID)
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

	items, err := svc.ListImportItems(ctx, created.ID, testGAEBImportCompanyID)
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

	failed, err := svc.MarkImportFailed(ctx, created.ID, "parser-v1", "x83", "GAEB-Struktur konnte nicht vollständig gelesen werden", testGAEBImportCompanyID)
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

	itemsAfterFailed, err := svc.ListImportItems(ctx, created.ID, testGAEBImportCompanyID)
	if err != nil {
		t.Fatalf("list import items after failed transition: %v", err)
	}
	if len(itemsAfterFailed) != 0 {
		t.Fatalf("expected no import items after failed transition, got %d", len(itemsAfterFailed))
	}
}
