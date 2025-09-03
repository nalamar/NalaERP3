package projects

import (
    "context"
    "database/sql"
    "fmt"
    "strings"
    _ "modernc.org/sqlite"
    "github.com/google/uuid"
    "encoding/json"
    "nalaerp3/internal/settings"
    "os"
    "io"
)

// ImportLogikal importiert ein Logikal SQLite-Projekt in das Nala-Projektschema.
// Erwartet eine SQLite-Datei (Logikal Export) und legt ein Projekt mit Phasen (ElevationsGroup),
// Elevations (Positionen) und SingleElevations (Varianten) an. Materiallisten werden vorbereitet,
// jedoch zunächst nicht importiert (kann später erweitert werden).
func (s *Service) ImportLogikal(ctx context.Context, sqlitePath string, sourceName string) (*Project, string, error) {
    // Quick magic header check to provide clearer error messages
    if f, err := os.Open(sqlitePath); err == nil {
        header := make([]byte, 16)
        n, _ := io.ReadFull(f, header)
        _ = f.Close()
        if n < 16 || string(header) != "SQLite format 3\x00" {
            return nil, "", fmt.Errorf("die Datei ist keine gültige SQLite-Datenbank (erwartet Logikal-Export)")
        }
    }
    db, err := sql.Open("sqlite", sqlitePath)
    if err != nil { return nil, "", err }
    defer db.Close()

    // Metadaten Projekt
    var (
        projName, projOfferNo, projOrderNo, projGUID string
    )
    _ = db.QueryRowContext(ctx, `SELECT COALESCE(Name,''), COALESCE(OfferNo,''), COALESCE(OrderNo,''), COALESCE(xGUID,'') FROM Projects LIMIT 1`).Scan(&projName, &projOfferNo, &projOrderNo, &projGUID)
    nummer := strings.TrimSpace(projOfferNo)
    if nummer == "" { nummer = strings.TrimSpace(projOrderNo) }
    if nummer == "" { nummer = projGUID }
    if nummer == "" {
        // Verwende Nummernkreis 'project'
        if n, e := settings.NewNumberingService(s.pg).Next(ctx, "project"); e == nil { nummer = n } else { nummer = "PRJ-" }
    }

    // Projekt finden oder anlegen (Re-Import)
    var p *Project
    // try find existing by nummer
    var existingID string
    if err := s.pg.QueryRow(ctx, `SELECT id FROM projects WHERE nummer=$1`, nummer).Scan(&existingID); err == nil && existingID != "" {
        // update name/status
        if _, err := s.pg.Exec(ctx, `UPDATE projects SET name=$1, status=$2 WHERE id=$3`, defaultString(projName, "Logikal-Projekt"), "importiert", existingID); err != nil {
            return nil, "", fmt.Errorf("projekt update: %w", err)
        }
        p, err = s.Get(ctx, existingID)
        if err != nil { return nil, "", fmt.Errorf("projekt laden: %w", err) }
    } else {
        p, err = s.Create(ctx, ProjectCreate{ Nummer: nummer, Name: defaultString(projName, "Logikal-Projekt"), Status: "importiert" })
        if err != nil { return nil, "", fmt.Errorf("projekt anlegen: %w", err) }
    }

    // Import-Run anlegen
    importID := uuidStr()
    if _, err := s.pg.Exec(ctx, `INSERT INTO project_imports (id, project_id, source) VALUES ($1,$2,$3)`, importID, p.ID, sourceName); err != nil {
        return nil, "", fmt.Errorf("import-run anlegen: %w", err)
    }
    // Hilfsfunktionen für Log
    type counters struct{
        createdPhases, updatedPhases int
        createdElevs, updatedElevs int
        createdVars, updatedVars, deletedVars int
        materialsReplaced int
    }
    cnt := &counters{}
    logChange := func(kind, action, internalID, externalRef, msg string, before, after any) {
        var braw, araw []byte
        if before != nil { braw, _ = json.Marshal(before) }
        if after != nil { araw, _ = json.Marshal(after) }
        _, _ = s.pg.Exec(ctx, `INSERT INTO project_import_changes (id, import_id, kind, action, internal_id, external_ref, message, before_data, after_data) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, uuidStr(), importID, kind, action, nullIfEmpty(internalID), nullIfEmpty(externalRef), nullIfEmpty(msg), braw, araw)
    }

    // snapshot helpers
    snapPhase := func(id string) map[string]any {
        var p Phase
        if err := s.pg.QueryRow(ctx, `SELECT id, project_id, nummer, name, COALESCE(beschreibung,''), sort_order FROM project_phases WHERE id=$1`, id).Scan(&p.ID, &p.ProjectID, &p.Nummer, &p.Name, &p.Beschreibung, &p.SortOrder); err == nil {
            return map[string]any{"id": p.ID, "project_id": p.ProjectID, "nummer": p.Nummer, "name": p.Name, "beschreibung": p.Beschreibung, "sort_order": p.SortOrder}
        }
        return nil
    }
    snapElevation := func(id string) map[string]any {
        var e Elevation
        if err := s.pg.QueryRow(ctx, `SELECT id, phase_id, nummer, name, COALESCE(beschreibung,''), menge, width_mm, height_mm, COALESCE(external_guid,'') FROM project_elevations WHERE id=$1`, id).Scan(&e.ID, &e.PhaseID, &e.Nummer, &e.Name, &e.Beschreibung, &e.Menge, &e.WidthMM, &e.HeightMM, &e.ExternalGUID); err == nil {
            m := map[string]any{"id": e.ID, "phase_id": e.PhaseID, "nummer": e.Nummer, "name": e.Name, "beschreibung": e.Beschreibung, "menge": e.Menge, "external_guid": e.ExternalGUID}
            if e.WidthMM != nil { m["width_mm"] = *e.WidthMM }
            if e.HeightMM != nil { m["height_mm"] = *e.HeightMM }
            return m
        }
        return nil
    }
    snapVariant := func(id string) map[string]any {
        var se SingleElevation
        if err := s.pg.QueryRow(ctx, `SELECT id, elevation_id, name, COALESCE(beschreibung,''), menge, selected, COALESCE(external_guid,'') FROM project_single_elevations WHERE id=$1`, id).Scan(&se.ID, &se.ElevationID, &se.Name, &se.Beschreibung, &se.Menge, &se.Selected, &se.ExternalGUID); err == nil {
            return map[string]any{"id": se.ID, "elevation_id": se.ElevationID, "name": se.Name, "beschreibung": se.Beschreibung, "menge": se.Menge, "selected": se.Selected, "external_guid": se.ExternalGUID}
        }
        return nil
    }
    snapMaterials := func(singleID string) map[string]any {
        // profiles
        rows, _ := s.pg.Query(ctx, `SELECT supplier_code, article_code, description, length_mm, qty, unit FROM single_elevation_profiles WHERE single_elevation_id=$1`, singleID)
        profs := make([]map[string]any, 0)
        for rows != nil && rows.Next() {
            var sc, ac, d, unit string; var l *float64; var q float64
            _ = rows.Scan(&sc, &ac, &d, &l, &q, &unit)
            m := map[string]any{"supplier_code": sc, "article_code": ac, "description": d, "qty": q, "unit": unit}
            if l != nil { m["length_mm"] = *l }
            profs = append(profs, m)
        }
        if rows != nil { rows.Close() }
        // articles
        rows, _ = s.pg.Query(ctx, `SELECT supplier_code, article_code, description, qty, unit FROM single_elevation_articles WHERE single_elevation_id=$1`, singleID)
        arts := make([]map[string]any, 0)
        for rows != nil && rows.Next() {
            var sc, ac, d, unit string; var q float64
            _ = rows.Scan(&sc, &ac, &d, &q, &unit)
            arts = append(arts, map[string]any{"supplier_code": sc, "article_code": ac, "description": d, "qty": q, "unit": unit})
        }
        if rows != nil { rows.Close() }
        // glass
        rows, _ = s.pg.Query(ctx, `SELECT configuration, description, width_mm, height_mm, area_m2, qty, unit FROM single_elevation_glass WHERE single_elevation_id=$1`, singleID)
        gls := make([]map[string]any, 0)
        for rows != nil && rows.Next() {
            var conf, d, unit string; var w, h, a *float64; var q float64
            _ = rows.Scan(&conf, &d, &w, &h, &a, &q, &unit)
            m := map[string]any{"configuration": conf, "description": d, "qty": q, "unit": unit}
            if w != nil { m["width_mm"] = *w }
            if h != nil { m["height_mm"] = *h }
            if a != nil { m["area_m2"] = *a }
            gls = append(gls, m)
        }
        if rows != nil { rows.Close() }
        return map[string]any{"profiles": profs, "articles": arts, "glass": gls}
    }

    // Phasen/Lose über ElevationGroups.PhaseId ableiten (stabilisiert: ein Los kann mehrere Positionen enthalten)
    // Map: groupID -> phaseId
    groupToPhase := make(map[int64]int64)
    phaseIDs := make(map[int64]struct{})
    if rows, err := db.QueryContext(ctx, `SELECT ElevationGroupID, PhaseId FROM ElevationGroups`); err == nil {
        for rows.Next() {
            var gid, pid sql.NullInt64
            if e := rows.Scan(&gid, &pid); e == nil {
                if gid.Valid && pid.Valid { groupToPhase[gid.Int64] = pid.Int64; phaseIDs[pid.Int64] = struct{}{} }
            }
        }
        _ = rows.Close()
    }

    // Optional: Phasen-Namen aus Phases-Tabelle lesen, wenn vorhanden
    namesByPhaseID := make(map[int64]string)
    if rows, err := db.QueryContext(ctx, `SELECT PhaseId, COALESCE(Name,'') FROM Phases`); err == nil {
        for rows.Next() {
            var pid sql.NullInt64; var name sql.NullString
            if e := rows.Scan(&pid, &name); e == nil {
                if pid.Valid && name.Valid {
                    n := strings.TrimSpace(name.String)
                    if n != "" { namesByPhaseID[pid.Int64] = n }
                }
            }
        }
        _ = rows.Close()
    }

    phaseByPhaseID := make(map[int64]*Phase)
    // Wenn keine PhaseIds gefunden: eine Standard-Phase anlegen
    if len(phaseIDs) == 0 {
        ph, err := s.CreatePhase(ctx, p.ID, PhaseCreate{ Nummer: "1", Name: "Los 1", SortOrder: 0 })
        if err != nil { return nil, "", fmt.Errorf("standard-phase: %w", err) }
        // Markiere -1 als Standard
        phaseByPhaseID[-1] = ph
        cnt.createdPhases++
        logChange("phase", "created", ph.ID, "phase:default", "Phase erstellt: Los 1", nil, snapPhase(ph.ID))
    } else {
        // Phasen je PhaseId anlegen/aktualisieren
        i := 0
        for pid := range phaseIDs {
            num := fmt.Sprintf("%d", pid)
            desiredName := fmt.Sprintf("Los %d", pid)
            if n, ok := namesByPhaseID[pid]; ok && strings.TrimSpace(n) != "" { desiredName = n }
            var ph Phase
            if err := s.pg.QueryRow(ctx, `SELECT id, project_id, nummer, name, COALESCE(beschreibung,''), sort_order, angelegt_am FROM project_phases WHERE project_id=$1 AND nummer=$2`, p.ID, num).Scan(
                &ph.ID, &ph.ProjectID, &ph.Nummer, &ph.Name, &ph.Beschreibung, &ph.SortOrder, &ph.Angelegt,
            ); err == nil {
                before := snapPhase(ph.ID)
                _, _ = s.pg.Exec(ctx, `UPDATE project_phases SET name=$1, sort_order=$2 WHERE id=$3`, desiredName, i, ph.ID)
                phaseByPhaseID[pid] = &ph
                cnt.updatedPhases++
                after := snapPhase(ph.ID)
                logChange("phase", "updated", ph.ID, fmt.Sprintf("phase:%d", pid), fmt.Sprintf("Phase aktualisiert: %s", desiredName), before, after)
            } else {
                ph2, err := s.CreatePhase(ctx, p.ID, PhaseCreate{ Nummer: num, Name: desiredName, SortOrder: i })
                if err != nil { return nil, "", fmt.Errorf("phase %d: %w", pid, err) }
                phaseByPhaseID[pid] = ph2
                cnt.createdPhases++
                logChange("phase", "created", ph2.ID, fmt.Sprintf("phase:%d", pid), fmt.Sprintf("Phase erstellt: %s", desiredName), nil, snapPhase(ph2.ID))
            }
            i++
        }
    }

    // Elevations: erst erfassen, um Main je Group zu finden (Alternative==0)
    type elevRow struct {
        id int64; groupID int64; name string; guid string; amount float64; width sql.NullFloat64; height sql.NullFloat64; alt sql.NullInt64
    }
    rows, err := db.QueryContext(ctx, `SELECT ElevationID, ElevationGroupId, COALESCE(Name,''), COALESCE(xGUID,''), COALESCE(Amount,0), Width, Height, Alternative FROM Elevations`)
    if err != nil { return nil, "", fmt.Errorf("elevations lesen: %w", err) }
    elevs := make([]elevRow, 0)
    for rows.Next() {
        var r elevRow
        if err := rows.Scan(&r.id, &r.groupID, &r.name, &r.guid, &r.amount, &r.width, &r.height, &r.alt); err != nil { return nil, "", err }
        elevs = append(elevs, r)
    }
    _ = rows.Close()

    // je Phase Positionsnummer zählen
    seq := make(map[string]int)
    elevIDMap := make(map[int64]string)        // ext ElevationID -> internal elevation UUID
    mainElevationByGroup := make(map[int64]string) // group -> internal elevation UUID (Alternative==0 bevorzugt)
    keptElevations := make(map[string]struct{}) // alle in diesem Lauf angelegten/aktualisierten Elevations
    seenGuids := make(map[string]struct{})      // alle gesehenen External GUIDs aus diesem Import

    for _, r := range elevs {
        // Phase bestimmen: aus groupToPhase -> phaseByPhaseID, sonst Standard (-1)
        var ph *Phase
        if pid, ok := groupToPhase[r.groupID]; ok {
            ph = phaseByPhaseID[pid]
        } else {
            ph = phaseByPhaseID[-1]
        }
        if ph == nil { continue }
        // find existing by external_guid if present
        var elevID string
        if strings.TrimSpace(r.guid) != "" {
            _ = s.pg.QueryRow(ctx, `SELECT id FROM project_elevations WHERE phase_id=$1 AND external_guid=$2`, ph.ID, r.guid).Scan(&elevID)
            seenGuids[strings.TrimSpace(r.guid)] = struct{}{}
        }
        // fallback: by name within phase
        if elevID == "" && strings.TrimSpace(r.name) != "" {
            _ = s.pg.QueryRow(ctx, `SELECT id FROM project_elevations WHERE phase_id=$1 AND name=$2 LIMIT 1`, ph.ID, r.name).Scan(&elevID)
        }
        var w, h *float64
        if r.width.Valid { w = &r.width.Float64 }
        if r.height.Valid { h = &r.height.Float64 }
        if elevID == "" {
            // assign running number within phase
            seq[ph.ID]++
            n := fmt.Sprintf("%d", seq[ph.ID])
            e, err := s.CreateElevation(ctx, ph.ID, ElevationCreate{ Nummer: n, Name: defaultString(r.name, fmt.Sprintf("Pos %s", n)), Beschreibung: "", Menge: r.amount, WidthMM: w, HeightMM: h, ExternalGUID: r.guid })
            if err != nil { return nil, "", fmt.Errorf("elevation anlegen: %w", err) }
            elevID = e.ID
            cnt.createdElevs++
            logChange("elevation", "created", elevID, r.guid, fmt.Sprintf("Position erstellt: %s", e.Name), nil, snapElevation(elevID))
        } else {
            // update
            before := snapElevation(elevID)
            _, _ = s.pg.Exec(ctx, `UPDATE project_elevations SET name=$1, beschreibung=$2, menge=$3, width_mm=$4, height_mm=$5, external_guid=$6 WHERE id=$7`, defaultString(r.name, "Position"), "", r.amount, w, h, nullIfEmpty(r.guid), elevID)
            cnt.updatedElevs++
            after := snapElevation(elevID)
            logChange("elevation", "updated", elevID, r.guid, fmt.Sprintf("Position aktualisiert: %s", defaultString(r.name, "Position")), before, after)
        }
        keptElevations[elevID] = struct{}{}
        elevIDMap[r.id] = elevID
        if (r.alt.Valid && r.alt.Int64 == 0) || mainElevationByGroup[r.groupID] == "" {
            mainElevationByGroup[r.groupID] = elevID
        }
    }

    // SingleElevations -> Varianten an die Hauptelevation der Gruppe hängen
    srows, err := db.QueryContext(ctx, `SELECT SingleElevationID, ElevationGroupId, COALESCE(Name,''), COALESCE(xGUID,''), COALESCE(Amount,0), COALESCE(SystemLongName,''), COALESCE(SystemName,''), COALESCE(SystemCode,''), COALESCE(Picture1_File,'') FROM SingleElevations`)
    if err != nil { return nil, "", fmt.Errorf("single elevations lesen: %w", err) }
    singleByGUID := make(map[string]string) // xGUID -> internal single_elevation id
    seriesByGroup := make(map[int64]string) // ElevationGroupId -> Serie
    keptSingles := make(map[string]struct{}) // track kept ids per import
    for srows.Next() {
        var sid, gid int64; var name, guid string; var amt float64; var sysLong, sysName, sysCode, pic1 string
        if err := srows.Scan(&sid, &gid, &name, &guid, &amt, &sysLong, &sysName, &sysCode, &pic1); err != nil { return nil, "", err }
        elevID := mainElevationByGroup[gid]
        if elevID == "" { continue }
        // find existing by guid
        var seID string
        if strings.TrimSpace(guid) != "" { _ = s.pg.QueryRow(ctx, `SELECT id FROM project_single_elevations WHERE elevation_id=$1 AND external_guid=$2`, elevID, guid).Scan(&seID) }
        if seID == "" && strings.TrimSpace(name) != "" { _ = s.pg.QueryRow(ctx, `SELECT id FROM project_single_elevations WHERE elevation_id=$1 AND name=$2 LIMIT 1`, elevID, name).Scan(&seID) }
        if seID == "" {
            se, err := s.CreateSingleElevation(ctx, elevID, SingleElevationCreate{ Name: defaultString(name, "Variante"), Menge: amt, Selected: false, ExternalGUID: guid })
            if err != nil { return nil, "", fmt.Errorf("variante anlegen: %w", err) }
            seID = se.ID
            cnt.createdVars++
            logChange("variant", "created", seID, guid, fmt.Sprintf("Variante erstellt: %s", se.Name), nil, snapVariant(seID))
        } else {
            before := snapVariant(seID)
            _, _ = s.pg.Exec(ctx, `UPDATE project_single_elevations SET name=$1, beschreibung=$2, menge=$3, external_guid=$4 WHERE id=$5`, defaultString(name, "Variante"), "", amt, nullIfEmpty(guid), seID)
            cnt.updatedVars++
            after := snapVariant(seID)
            logChange("variant", "updated", seID, guid, fmt.Sprintf("Variante aktualisiert: %s", defaultString(name, "Variante")), before, after)
        }
        if guid != "" { singleByGUID[guid] = seID }
        keptSingles[seID] = struct{}{}

        // Serie merken (bevorzugt LongName, sonst Name, sonst Code)
        if _, ok := seriesByGroup[gid]; !ok {
            s := strings.TrimSpace(sysLong)
            if s == "" { s = strings.TrimSpace(sysName) }
            if s == "" { s = strings.TrimSpace(sysCode) }
            if s != "" { seriesByGroup[gid] = s }
        }

        // Bildpfad relativ machen und auf Position speichern
        if strings.TrimSpace(pic1) != "" {
            rp := toRelativeAssetPath(pic1)
            if rp != "" {
                _, _ = s.pg.Exec(ctx, `UPDATE project_elevations SET picture1_relpath=$1 WHERE id=$2`, rp, elevID)
            }
        }
    }
    _ = srows.Close()

    // Serien auf Hauptelevation schreiben
    for gid, serie := range seriesByGroup {
        if eid := mainElevationByGroup[gid]; eid != "" {
            _, _ = s.pg.Exec(ctx, `UPDATE project_elevations SET serie=$1 WHERE id=$2`, serie, eid)
        }
    }

    // Entferne Varianten, die nicht mehr existieren (nur für Elevations, die wir referenziert haben)
    for _, elevID := range mainElevationByGroup {
        rows, err := s.pg.Query(ctx, `SELECT id FROM project_single_elevations WHERE elevation_id=$1`, elevID)
        if err == nil {
            for rows.Next() {
                var id string
                if err := rows.Scan(&id); err == nil {
                    if _, ok := keptSingles[id]; !ok {
                        before := snapVariant(id)
                        _, _ = s.pg.Exec(ctx, `DELETE FROM project_single_elevations WHERE id=$1`, id)
                        cnt.deletedVars++
                        logChange("variant", "deleted", id, "", "Variante gelöscht (nicht mehr im Export)", before, nil)
                    }
                }
            }
            rows.Close()
        }
    }

    // Oberflächen aus ElevationSurface lesen und auf Positionen schreiben
    if rows, err := db.QueryContext(ctx, `SELECT ElevationId, COALESCE(SurfaceColor,'') FROM ElevationSurface`); err == nil {
        written := make(map[string]struct{})
        for rows.Next() {
            var extEID int64; var color string
            if err := rows.Scan(&extEID, &color); err != nil { return nil, "", err }
            if strings.TrimSpace(color) == "" { continue }
            if internalID, ok := elevIDMap[extEID]; ok && internalID != "" {
                if _, done := written[internalID]; done { continue }
                _, _ = s.pg.Exec(ctx, `UPDATE project_elevations SET oberflaeche=$1 WHERE id=$2`, color, internalID)
                written[internalID] = struct{}{}
            }
        }
        _ = rows.Close()
    }

    // Materiallisten je Variante (über Insertions verknüpfen: Insertions.xGUID == SingleElevations.xGUID)
    // 1) Map xGUID -> InsertionID
    insRows, err := db.QueryContext(ctx, `SELECT InsertionID, COALESCE(xGUID,'' ) FROM Insertions`)
    if err == nil {
        insByGUID := make(map[string]int64)
        for insRows.Next() {
            var iid int64; var guid string
            if err := insRows.Scan(&iid, &guid); err != nil { return nil, "", err }
            if guid != "" { insByGUID[guid] = iid }
        }
        _ = insRows.Close()

        // Build reverse: insertionID -> single_elevation_id using xGUID
        seByInsertion := make(map[int64]string)
        insGroup := make(map[int64]int64)
        // Re-read Insertions to also get GroupID for fallback mapping
        if insRows2, e2 := db.QueryContext(ctx, `SELECT InsertionID, COALESCE(xGUID,''), COALESCE(GroupID,0) FROM Insertions`); e2 == nil {
            for insRows2.Next() {
                var iid int64; var guid string; var gid int64
                if err := insRows2.Scan(&iid, &guid, &gid); err == nil { insGroup[iid] = gid; if guid != "" { if seID, ok := singleByGUID[guid]; ok { seByInsertion[iid] = seID } } }
            }
            _ = insRows2.Close()
        } else {
            for guid, iid := range insByGUID { if seID, ok := singleByGUID[guid]; ok { seByInsertion[iid] = seID } }
        }

        // helper: ensure seID for insertion (fallback über GroupID -> Elevation -> erste Variante)
        getSeForInsertion := func(insID int64) string {
            if seID, ok := seByInsertion[insID]; ok && seID != "" { return seID }
            if gid, ok := insGroup[insID]; ok {
                if elevID := mainElevationByGroup[gid]; elevID != "" {
                    var found string
                    _ = s.pg.QueryRow(ctx, `SELECT id FROM project_single_elevations WHERE elevation_id=$1 ORDER BY angelegt_am ASC LIMIT 1`, elevID).Scan(&found)
                    if found != "" { return found }
                    if se, err := s.CreateSingleElevation(ctx, elevID, SingleElevationCreate{ Name: "Variante", Menge: 1, Selected: false }); err == nil { return se.ID }
                }
            }
            return ""
        }

        // 2) Profiles (write now with reverse map). Vor dem Insert die bisherigen Materialien pro Variante löschen (idempotent)
        autoLink := func(kind, itemID, supplier, code string) {
            code = strings.TrimSpace(code)
            if code == "" { return }
            var mid string
            // 1) exakter Match über Material-Nummer = Artikelcode
            _ = s.pg.QueryRow(ctx, `SELECT id FROM materials WHERE nummer=$1`, code).Scan(&mid)
            if strings.TrimSpace(mid) == "" { return }
            switch strings.ToLower(kind) {
            case "profiles": _, _ = s.pg.Exec(ctx, `UPDATE single_elevation_profiles SET material_id=$1 WHERE id=$2`, mid, itemID)
            case "articles": _, _ = s.pg.Exec(ctx, `UPDATE single_elevation_articles SET material_id=$1 WHERE id=$2`, mid, itemID)
            case "glass":    _, _ = s.pg.Exec(ctx, `UPDATE single_elevation_glass SET material_id=$1 WHERE id=$2`, mid, itemID)
            }
        }
        cleared := make(map[string]struct{})
        if rows, err := db.QueryContext(ctx, `SELECT InsertionId, COALESCE(ArticleCode,''), COALESCE(Description,''), COALESCE(CAST(LK_SupplierID AS TEXT),''), COALESCE(Length,0), COALESCE(Amount,0), COALESCE(Length_Unit,'') FROM Profiles`); err == nil {
            for rows.Next() {
                var insID int64; var code, desc, supplier string; var length float64; var amt int64; var unit string
                if err := rows.Scan(&insID, &code, &desc, &supplier, &length, &amt, &unit); err != nil { return nil, "", err }
                seID := getSeForInsertion(insID); if seID == "" { continue }
                if _, done := cleared[seID]; !done {
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_profiles WHERE single_elevation_id=$1`, seID)
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_articles WHERE single_elevation_id=$1`, seID)
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_glass WHERE single_elevation_id=$1`, seID)
                    cleared[seID] = struct{}{}
                    before := snapMaterials(seID)
                    cnt.materialsReplaced++
                    logChange("materials", "replaced", seID, "", "Materialliste ersetzt (Profile/Artikel/Glas neu aufgebaut)", before, nil)
                }
                // Insert into single_elevation_profiles
                iid := uuidStr()
                if _, err := s.pg.Exec(ctx, `INSERT INTO single_elevation_profiles (id, single_elevation_id, supplier_code, article_code, description, length_mm, qty, unit) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
                    iid, seID, nullIfEmpty(supplier), nullIfEmpty(code), nullIfEmpty(desc), nullIfZeroFloat(length), nullIfZeroInt(amt), nullIfEmpty(unit)); err != nil { return nil, "", err }
                autoLink("profiles", iid, supplier, code)
            }
            _ = rows.Close()
        }

        // 3) Articles
        if rows, err := db.QueryContext(ctx, `SELECT InsertionId, COALESCE(ArticleCode,''), COALESCE(Description,''), COALESCE(CAST(LK_SupplierId AS TEXT),''), COALESCE(Amount,0), COALESCE(Units_Unit,'') FROM Articles`); err == nil {
            for rows.Next() {
                var insID int64; var code, desc, supplier string; var amt int64; var unit string
                if err := rows.Scan(&insID, &code, &desc, &supplier, &amt, &unit); err != nil { return nil, "", err }
                seID := getSeForInsertion(insID); if seID == "" { continue }
                if _, done := cleared[seID]; !done {
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_profiles WHERE single_elevation_id=$1`, seID)
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_articles WHERE single_elevation_id=$1`, seID)
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_glass WHERE single_elevation_id=$1`, seID)
                    cleared[seID] = struct{}{}
                    before := snapMaterials(seID)
                    cnt.materialsReplaced++
                    logChange("materials", "replaced", seID, "", "Materialliste ersetzt (Profile/Artikel/Glas neu aufgebaut)", before, nil)
                }
                iid := uuidStr()
                if _, err := s.pg.Exec(ctx, `INSERT INTO single_elevation_articles (id, single_elevation_id, supplier_code, article_code, description, qty, unit) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
                    iid, seID, nullIfEmpty(supplier), nullIfEmpty(code), nullIfEmpty(desc), nullIfZeroInt(amt), nullIfEmpty(unit)); err != nil { return nil, "", err }
                autoLink("articles", iid, supplier, code)
            }
            _ = rows.Close()
        }

        // 4) Glass
        if rows, err := db.QueryContext(ctx, `SELECT InsertionId, COALESCE(Configuration,''), COALESCE(Description,''), 1 FROM Glass`); err == nil {
            for rows.Next() {
                var insID int64; var conf, desc string; var qty int
                if err := rows.Scan(&insID, &conf, &desc, &qty); err != nil { return nil, "", err }
                seID := getSeForInsertion(insID); if seID == "" { continue }
                if _, done := cleared[seID]; !done {
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_profiles WHERE single_elevation_id=$1`, seID)
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_articles WHERE single_elevation_id=$1`, seID)
                    _, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_glass WHERE single_elevation_id=$1`, seID)
                    cleared[seID] = struct{}{}
                    before := snapMaterials(seID)
                    cnt.materialsReplaced++
                    logChange("materials", "replaced", seID, "", "Materialliste ersetzt (Profile/Artikel/Glas neu aufgebaut)", before, nil)
                }
                iid := uuidStr()
                if _, err := s.pg.Exec(ctx, `INSERT INTO single_elevation_glass (id, single_elevation_id, configuration, description, qty) VALUES ($1,$2,$3,$4,$5)`,
                    iid, seID, nullIfEmpty(conf), nullIfEmpty(desc), qty); err != nil { return nil, "", err }
                // optional: autoLink("glass", iid, "", conf) // bewusst weggelassen, um Fehlverknüpfungen zu vermeiden
            }
            _ = rows.Close()
        }
    }

    // Bereinigung: veraltete Positionen in betroffenen Losen entfernen
    // 1) In allen in diesem Import referenzierten Losen alle Elevations löschen, die nicht (mehr) importiert wurden
    currentPhaseIDs := make([]string, 0, len(phaseByPhaseID))
    currentPhaseNums := make(map[string]struct{})
    if len(phaseByPhaseID) == 0 {
        // Sonderfall Standard-Los "1"
        currentPhaseNums["1"] = struct{}{}
    }
    for pid, ph := range phaseByPhaseID {
        if ph != nil { currentPhaseIDs = append(currentPhaseIDs, ph.ID) }
        // Phase-Nummern für spätere Phasenbereinigung merken
        num := fmt.Sprintf("%d", pid)
        if pid == -1 { num = "1" }
        currentPhaseNums[num] = struct{}{}
    }
    for _, phID := range currentPhaseIDs {
        rows, _ := s.pg.Query(ctx, `SELECT id FROM project_elevations WHERE phase_id=$1`, phID)
        if rows != nil {
            for rows.Next() {
                var id string
                if err := rows.Scan(&id); err == nil {
                    if _, ok := keptElevations[id]; !ok {
                        before := snapElevation(id)
                        _, _ = s.pg.Exec(ctx, `DELETE FROM project_elevations WHERE id=$1`, id)
                        logChange("elevation", "deleted", id, "", "Position gelöscht (nicht mehr im Export)", before, nil)
                    }
                }
            }
            rows.Close()
        }
    }

    // 2) Doppelte/alte Elevations mit gleicher External GUID in anderen Losen entfernen
    if len(seenGuids) > 0 {
        rows, _ := s.pg.Query(ctx, `SELECT e.id, COALESCE(e.external_guid,'') FROM project_elevations e JOIN project_phases ph ON ph.id=e.phase_id WHERE ph.project_id=$1`, p.ID)
        if rows != nil {
            for rows.Next() {
                var id, guid string
                if err := rows.Scan(&id, &guid); err == nil {
                    if strings.TrimSpace(guid) != "" {
                        if _, seen := seenGuids[strings.TrimSpace(guid)]; seen {
                            if _, kept := keptElevations[id]; !kept {
                                before := snapElevation(id)
                                _, _ = s.pg.Exec(ctx, `DELETE FROM project_elevations WHERE id=$1`, id)
                                logChange("elevation", "deleted", id, guid, "Position gelöscht (alte Zuordnung, durch Re-Import ersetzt)", before, nil)
                            }
                        }
                    }
                }
            }
            rows.Close()
        }
    }

    // 3) Lose (Phasen) entfernen, die im aktuellen Import nicht mehr vorkommen (numerische Nummern)
    rowsPh, _ := s.pg.Query(ctx, `SELECT id, nummer, name FROM project_phases WHERE project_id=$1`, p.ID)
    if rowsPh != nil {
        for rowsPh.Next() {
            var id, num, name string
            if err := rowsPh.Scan(&id, &num, &name); err == nil {
                numTrim := strings.TrimSpace(num)
                // nur numerische Phasen-Nummern betrachten
                isNumeric := true
                for _, ch := range numTrim { if ch < '0' || ch > '9' { isNumeric = false; break } }
                if isNumeric {
                    if _, ok := currentPhaseNums[numTrim]; !ok {
                        before := snapPhase(id)
                        _, _ = s.pg.Exec(ctx, `DELETE FROM project_phases WHERE id=$1`, id)
                        logChange("phase", "deleted", id, fmt.Sprintf("phase:%s", numTrim), fmt.Sprintf("Phase gelöscht: %s", defaultString(name, "Los")), before, nil)
                    }
                }
            }
        }
        rowsPh.Close()
    }

    // Counters in import-run sichern
    _, _ = s.pg.Exec(ctx, `UPDATE project_imports SET created_phases=$1, updated_phases=$2, created_elevations=$3, updated_elevations=$4, created_variants=$5, updated_variants=$6, deleted_variants=$7, materials_replaced_variants=$8 WHERE id=$9`,
        cnt.createdPhases, cnt.updatedPhases, cnt.createdElevs, cnt.updatedElevs, cnt.createdVars, cnt.updatedVars, cnt.deletedVars, cnt.materialsReplaced, importID)

    return p, importID, nil
}

func defaultString(v, def string) string { v = strings.TrimSpace(v); if v == "" { return def }; return v }

func uuidStr() string { return uuid.NewString() }

func nullIfZeroFloat(v float64) any { if v == 0 { return nil }; return v }
func nullIfZeroInt(v int64) any { if v == 0 { return nil }; return v }

// toRelativeAssetPath schneidet absolute Pfade auf Emfs/... oder Rtfs/... zusammen
func toRelativeAssetPath(p string) string {
    s := strings.ReplaceAll(p, "\\", "/")
    i := strings.Index(strings.ToLower(s), "/emfs/")
    if i >= 0 { return s[i+1:] }
    j := strings.Index(strings.ToLower(s), "/rtfs/")
    if j >= 0 { return s[j+1:] }
    // manchmal direkt Emfs... ohne Slash
    if strings.HasPrefix(strings.ToLower(s), "emfs/") || strings.HasPrefix(strings.ToLower(s), "rtfs/") { return s }
    return ""
}
