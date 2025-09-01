package projects

import (
    "context"
    "database/sql"
    _ "modernc.org/sqlite"
    "strings"
)

// AnalyzeLogikal liest eine Logikal-SQLite-Datei und liefert eine Zusammenfassung,
// ohne Daten in die Postgres-DB zu schreiben.
func (s *Service) AnalyzeLogikal(ctx context.Context, sqlitePath string) (map[string]any, error) {
    db, err := sql.Open("sqlite", sqlitePath)
    if err != nil { return nil, err }
    defer db.Close()

    out := map[string]any{}
    // Projekt-Metadaten
    var name, offerNo, orderNo, guid, activeTitle string
    _ = db.QueryRowContext(ctx, `SELECT COALESCE(Name,''), COALESCE(OfferNo,''), COALESCE(OrderNo,''), COALESCE(xGUID,''), COALESCE(ActiveTitle,'') FROM Projects LIMIT 1`).Scan(&name, &offerNo, &orderNo, &guid, &activeTitle)
    out["project"] = map[string]any{"name": name, "offer_no": offerNo, "order_no": orderNo, "guid": guid, "active_title": activeTitle}

    // ElevationGroups: group -> phaseId
    grpToPhase := map[int64]int64{}
    phaseSet := map[int64]struct{}{}
    if rows, err := db.QueryContext(ctx, `SELECT ElevationGroupID, PhaseId FROM ElevationGroups`); err == nil {
        for rows.Next() {
            var gid, pid sql.NullInt64
            if e := rows.Scan(&gid, &pid); e == nil {
                if gid.Valid && pid.Valid { grpToPhase[gid.Int64] = pid.Int64; phaseSet[pid.Int64] = struct{}{} }
            }
        }
        _ = rows.Close()
    }

    // Elevations: sammeln pro PhaseId
    phases := map[int64]map[string]any{}
    if rows, err := db.QueryContext(ctx, `SELECT ElevationID, ElevationGroupId, COALESCE(Name,''), COALESCE(xGUID,''), COALESCE(Amount,0) FROM Elevations`); err == nil {
        for rows.Next() {
            var eid, gid sql.NullInt64
            var ename, eguid string
            var amt sql.NullFloat64
            _ = rows.Scan(&eid, &gid, &ename, &eguid, &amt)
            var pid int64 = -1
            if gid.Valid {
                if p, ok := grpToPhase[gid.Int64]; ok { pid = p }
            }
            ph := phases[pid]
            if ph == nil { ph = map[string]any{"phase_id": pid, "elevation_groups": map[int64]int{}, "elevations": 0}; phases[pid] = ph }
            // count elevations
            ph["elevations"] = ph["elevations"].(int) + 1
            // groups per phase
            if gid.Valid {
                eg := ph["elevation_groups"].(map[int64]int)
                eg[gid.Int64] = eg[gid.Int64] + 1
            }
        }
        _ = rows.Close()
    }

    // SingleElevations count per group
    singlesPerGroup := map[int64]int{}
    if rows, err := db.QueryContext(ctx, `SELECT ElevationGroupId, COUNT(1) FROM SingleElevations GROUP BY ElevationGroupId`); err == nil {
        for rows.Next() {
            var gid sql.NullInt64
            var c int
            _ = rows.Scan(&gid, &c)
            if gid.Valid { singlesPerGroup[gid.Int64] = c }
        }
        _ = rows.Close()
    }

    // Phase-Namen heuristisch aus Insertions.ActiveTitle ableiten (häufigster Titel je Phase)
    phaseName := map[int64]string{}
    if rows, err := db.QueryContext(ctx, `
        SELECT eg.PhaseId, COALESCE(i.ActiveTitle,''), COUNT(1)
        FROM Insertions i
        JOIN Elevations e ON i.ElevationId = e.ElevationID
        LEFT JOIN ElevationGroups eg ON e.ElevationGroupId = eg.ElevationGroupID
        GROUP BY eg.PhaseId, i.ActiveTitle
    `); err == nil {
        // track best name per phase by count
        type agg struct{ name string; cnt int }
        best := map[int64]agg{}
        for rows.Next() {
            var pid sql.NullInt64; var t string; var c int
            _ = rows.Scan(&pid, &t, &c)
            if !pid.Valid { continue }
            t = strings.TrimSpace(t)
            if t == "" { continue }
            b := best[pid.Int64]
            if c > b.cnt { best[pid.Int64] = agg{name: t, cnt: c} }
        }
        _ = rows.Close()
        for pid, b := range best { if strings.TrimSpace(b.name) != "" { phaseName[pid] = b.name } }
    }

    // Phasen in Liste umwandeln
    list := make([]map[string]any, 0, len(phases))
    for _, ph := range phases {
        // konvertiere groups map
        egMap := ph["elevation_groups"].(map[int64]int)
        groups := make([]map[string]any, 0, len(egMap))
        for gid, cnt := range egMap {
            groups = append(groups, map[string]any{"group_id": gid, "elevations": cnt, "single_elevations": singlesPerGroup[gid]})
        }
        ph["elevation_groups"] = groups
        if pid, _ := ph["phase_id"].(int64); pid != 0 {
            if nm, ok := phaseName[pid]; ok { ph["name"] = nm }
        }
        list = append(list, ph)
    }
    out["phases"] = list
    out["phase_count"] = len(list)
    out["notes"] = "Phasen (Lose) anhand ElevationGroups.PhaseId; -1 bedeutet unbekannt/Standard; Phase-Name heuristisch aus Insertions.ActiveTitle (häufigster Titel)"
    return out, nil
}
