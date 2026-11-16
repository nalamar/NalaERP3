package quotes

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

type importGroupRow struct {
	outline, parentOutline, kind, bezeichnung string
	sortOrder                                 int
}

func importGroupRows(t *testing.T, ctx context.Context, svc *Service, importID string) []importGroupRow {
	t.Helper()
	rows, err := svc.pg.Query(ctx, `
		SELECT g.outline_no, COALESCE(p.outline_no, ''), g.kind, g.bezeichnung, g.sort_order
		FROM quote_import_groups g LEFT JOIN quote_import_groups p ON p.id = g.parent_group_id
		WHERE g.import_id=$1 ORDER BY g.sort_order`, importID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []importGroupRow
	for rows.Next() {
		var r importGroupRow
		if err := rows.Scan(&r.outline, &r.parentOutline, &r.kind, &r.bezeichnung, &r.sortOrder); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

type importItemRow struct {
	position, shortText, longText, kind, base, groupOutline string
	lumpSum, optional                                       bool
}

func loadImportItemRow(t *testing.T, ctx context.Context, svc *Service, importID, position string) importItemRow {
	t.Helper()
	r := importItemRow{position: position}
	if err := svc.pg.QueryRow(ctx, `
		SELECT i.short_text, i.long_text, i.item_kind, i.base_position_no, COALESCE(g.outline_no, ''), i.is_lump_sum, i.is_optional
		FROM quote_import_items i LEFT JOIN quote_import_groups g ON g.id = i.group_id
		WHERE i.import_id=$1 AND i.position_no=$2`, importID, position).
		Scan(&r.shortText, &r.longText, &r.kind, &r.base, &r.groupOutline, &r.lumpSum, &r.optional); err != nil {
		t.Fatalf("Position %s: %v", position, err)
	}
	return r
}

func importCounts(t *testing.T, ctx context.Context, svc *Service, importID string) (groups, items int, vorbemerkungen []string) {
	t.Helper()
	if err := svc.pg.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM quote_import_groups WHERE import_id=$1),
		       (SELECT count(*) FROM quote_import_items WHERE import_id=$1),
		       (SELECT vorbemerkungen FROM quote_imports WHERE id=$1)`, importID).Scan(&groups, &items, &vorbemerkungen); err != nil {
		t.Fatal(err)
	}
	return groups, items, vorbemerkungen
}

// TestProcessGAEBImportStoresStructure: echte X83 über den Adapter bis in
// die Datenbank (Migration 116, Backlog I.2.2a).
func TestProcessGAEBImportStoresStructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := os.ReadFile("eval/testdata/lv-02.x83")
	if err != nil {
		t.Fatal(err)
	}
	svc, created := createUploadedGAEBImportWithSource(t, ctx, GAEBDAXMLParser{}, string(raw), "lv-02.x83")
	processed, err := svc.ProcessGAEBImport(ctx, created.ID, testGAEBImportCompanyID)
	if err != nil {
		t.Fatal(err)
	}
	if processed.Status != "parsed" || processed.ParserVersion != "gaeb-da-xml-v1" || processed.DetectedFormat != "da_xml_3.3_x83" || processed.ItemCount != 26 {
		t.Fatalf("Import: %+v", processed)
	}
	groups := importGroupRows(t, ctx, svc, created.ID)
	if len(groups) == 0 || groups[0] != (importGroupRow{outline: "1", kind: "los", bezeichnung: "Fassade", sortOrder: 1}) ||
		groups[1] != (importGroupRow{outline: "1.01", parentOutline: "1", kind: "titel", bezeichnung: "Pfosten-Riegel-Fassade", sortOrder: 2}) {
		t.Fatalf("Gliederung: %+v", groups)
	}
	first := loadImportItemRow(t, ctx, svc, created.ID, "1.01.0010")
	if first.shortText != "Pfosten-Riegel-Fassade Aluminium" || first.longText == "" || first.kind != "normal" || first.groupOutline != "1.01" || first.optional {
		t.Fatalf("erste Position: %+v", first)
	}
	_, _, vorbemerkungen := importCounts(t, ctx, svc, created.ID)
	if len(vorbemerkungen) != 1 || vorbemerkungen[0] == "" {
		t.Fatalf("Vorbemerkungen: %q", vorbemerkungen)
	}
	// Alternative mit Grundposition, Pauschale
	var alt, grund string
	if err := svc.pg.QueryRow(ctx, `SELECT position_no, base_position_no FROM quote_import_items WHERE import_id=$1 AND item_kind='alternativposition' ORDER BY sort_order LIMIT 1`,
		created.ID).Scan(&alt, &grund); err != nil {
		t.Fatal(err)
	}
	if a, g := loadImportItemRow(t, ctx, svc, created.ID, alt), loadImportItemRow(t, ctx, svc, created.ID, grund); a.base != grund || !a.optional || g.kind != "grundposition" || g.optional {
		t.Fatalf("Alternative %+v, Grundposition %+v", a, g)
	}
	var lumpSums int
	if err := svc.pg.QueryRow(ctx, `SELECT count(*) FROM quote_import_items WHERE import_id=$1 AND is_lump_sum`, created.ID).Scan(&lumpSums); err != nil || lumpSums == 0 {
		t.Fatalf("Pauschalen: %d %v", lumpSums, err)
	}
	// erneutes Speichern ersetzt, statt zu verdoppeln
	result, err := (GAEBDAXMLParser{}).ParseGAEB(ctx, strings.NewReader(string(raw)), "lv-02.x83")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveImportParse(ctx, created.ID, result, testGAEBImportCompanyID); err != nil {
		t.Fatal(err)
	}
	if g, i, v := importCounts(t, ctx, svc, created.ID); g != len(groups) || i != 26 || len(v) != 1 {
		t.Fatalf("nach erneutem Speichern: %d Gruppen, %d Positionen, %q", g, i, v)
	}
}

func TestSaveImportParseValidatesStructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	svc, created := createUploadedGAEBImportForProcessingTest(t, ctx, nil)
	item := func(pos, outline, kind, base string) QuoteImportItemInput {
		return QuoteImportItemInput{PositionNo: pos, OutlineNo: outline, Description: "Text " + pos, Qty: 1, Unit: "Stk", ItemKind: kind, BasePositionNo: base}
	}
	ok := GAEBImportParseResult{
		ParserVersion: "v", DetectedFormat: "f",
		Groups: []QuoteImportGroupInput{{OutlineNo: "01", Kind: "titel", Bezeichnung: " Titel "}, {OutlineNo: "01.01", ParentOutlineNo: "01", Kind: "untertitel"}},
		Items: []QuoteImportItemInput{item("01.01.0010", "01.01", "grundposition", ""), item("01.01.0020", " 01.01 ", "alternativposition", " 01.01.0010 "),
			item("99.0010", "99", "", "")},
		Vorbemerkungen: []string{"Hinweis A", "Hinweis B"},
	}
	if _, err := svc.SaveImportParse(ctx, created.ID, ok, testGAEBImportCompanyID); err != nil {
		t.Fatal(err)
	}
	if g := importGroupRows(t, ctx, svc, created.ID); len(g) != 2 || g[0].bezeichnung != "Titel" || g[1] != (importGroupRow{outline: "01.01", parentOutline: "01", kind: "untertitel", sortOrder: 2}) {
		t.Fatalf("Gliederung: %+v", g)
	}
	if r := loadImportItemRow(t, ctx, svc, created.ID, "01.01.0020"); r.kind != "alternativposition" || r.base != "01.01.0010" || r.groupOutline != "01.01" {
		t.Fatalf("Alternative: %+v", r)
	}
	// ohne Art: normal; ohne passende Gruppe: keine Gruppe
	if r := loadImportItemRow(t, ctx, svc, created.ID, "99.0010"); r.kind != "normal" || r.groupOutline != "" {
		t.Fatalf("ohne Art/Gruppe: %+v", r)
	}
	if _, _, v := importCounts(t, ctx, svc, created.ID); len(v) != 2 || v[1] != "Hinweis B" {
		t.Fatalf("Vorbemerkungen: %q", v)
	}
	cases := []struct {
		name string
		edit func(r *GAEBImportParseResult)
		want string
	}{
		{"Gruppe ohne OZ", func(r *GAEBImportParseResult) { r.Groups[0].OutlineNo = " " }, "Gliederung ohne Ordnungszahl"},
		{"Gruppe doppelt", func(r *GAEBImportParseResult) { r.Groups[1].OutlineNo = "01" }, "Gliederung 01 ist doppelt"},
		{"Stufe", func(r *GAEBImportParseResult) { r.Groups[0].Kind = "kapitel" }, `unbekannte Stufe "kapitel"`},
		{"Eltern fehlt", func(r *GAEBImportParseResult) { r.Groups[1].ParentOutlineNo = "02" }, "übergeordneter Bereich 02 fehlt"},
		{"Eltern später", func(r *GAEBImportParseResult) { r.Groups[0], r.Groups[1] = r.Groups[1], r.Groups[0] }, "übergeordneter Bereich 01 fehlt"},
		{"Art", func(r *GAEBImportParseResult) { r.Items[0].ItemKind = "eventual" }, `unbekannte Positionsart "eventual"`},
		{"Alternative ohne Grund", func(r *GAEBImportParseResult) { r.Items[1].BasePositionNo = " " }, "Grundposition nur und immer"},
		{"Grund bei Normalposition", func(r *GAEBImportParseResult) { r.Items[2].BasePositionNo = "01.01.0010" }, "Grundposition nur und immer"},
	}
	for _, c := range cases {
		r := ok
		r.Groups = append([]QuoteImportGroupInput(nil), ok.Groups...)
		r.Items = append([]QuoteImportItemInput(nil), ok.Items...)
		r.Vorbemerkungen = []string{"verworfen"}
		c.edit(&r)
		if _, err := svc.SaveImportParse(ctx, created.ID, r, testGAEBImportCompanyID); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: %v, erwartet %q", c.name, err, c.want)
		}
		// abgelehnt: das frühere Ergebnis bleibt unverändert
		if g, i, v := importCounts(t, ctx, svc, created.ID); g != 2 || i != 3 || len(v) != 2 {
			t.Fatalf("%s: Ergebnis verändert: %d %d %q", c.name, g, i, v)
		}
	}
	// bisheriger Aufruf ohne Gliederung: Gruppen und Vorbemerkungen werden geleert
	if _, err := svc.SaveImportParseResult(ctx, created.ID, "v", "f", []QuoteImportItemInput{{PositionNo: "1", OutlineNo: "01", Description: "x", Unit: "Stk"}},
		testGAEBImportCompanyID); err != nil {
		t.Fatal(err)
	}
	if g, i, v := importCounts(t, ctx, svc, created.ID); g != 0 || i != 1 || v == nil || len(v) != 0 {
		t.Fatalf("ohne Gliederung: %d %d %q", g, i, v)
	}
	if r := loadImportItemRow(t, ctx, svc, created.ID, "1"); r.kind != "normal" || r.groupOutline != "" || r.base != "" {
		t.Fatalf("ohne Gliederung: %+v", r)
	}
	// Prüfungen der Migration greifen auch ohne Service
	for name, sql := range map[string]string{
		"Art":                 `UPDATE quote_import_items SET item_kind='eventual' WHERE import_id=$1`,
		"Alternative":         `UPDATE quote_import_items SET item_kind='alternativposition' WHERE import_id=$1`,
		"Grund":               `UPDATE quote_import_items SET base_position_no='0010' WHERE import_id=$1`,
		"Stufe":               `INSERT INTO quote_import_groups (id, import_id, outline_no, kind) VALUES (gen_random_uuid(), $1, '01', 'kapitel')`,
		"OZ leer":             `INSERT INTO quote_import_groups (id, import_id, outline_no, kind) VALUES (gen_random_uuid(), $1, ' ', 'titel')`,
		"ohne Vorbemerkungen": `UPDATE quote_imports SET vorbemerkungen=NULL WHERE id=$1`,
	} {
		if _, err := svc.pg.Exec(ctx, sql, created.ID); err == nil {
			t.Fatalf("Datenbank nimmt %s an", name)
		}
	}
	if _, err := svc.pg.Exec(ctx, `INSERT INTO quote_import_groups (id, import_id, outline_no, kind) VALUES (gen_random_uuid(), $1, '01', 'titel'), (gen_random_uuid(), $1, '01', 'titel')`,
		created.ID); err == nil {
		t.Fatal("Datenbank nimmt doppelte OZ an")
	}
}
