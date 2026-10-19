package assets

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Pruefstand aus der Datenbank, betriebsweite Faelligkeitsliste und die
// Pruefung bei der Ausgabe (ADR 0028, Backlog F.3.5.2).
//
// Drei Sichten auf dieselben Regeln: der Stand EINES Assets, die Liste
// aller offenen Punkte des Betriebs und der Stand am Ausgabetag. Alle drei
// laufen ueber standingAt, damit sie nicht auseinanderlaufen;
// TestInspectionDueListMatchesStanding wacht darueber.

// queryer ist das, was Pool und Transaktion gemeinsam haben.
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// standingAt wertet die Pruefungen eines Assets am Tag day aus. Es
// zaehlen nur Pruefungen bis einschliesslich day: am Ausgabetag einer
// nachgetragenen Ausgabe war eine spaetere Pruefung noch nicht gemacht.
// Fuer "heute" aendert das nichts, weil Pruefungen nicht in der Zukunft
// liegen duerfen.
func standingAt(records []Inspection, pruefintervallMonate *int, day time.Time, warnDays int) InspectionStanding {
	upTo := make([]Inspection, 0, len(records))
	for _, r := range records {
		if !dateOnly(r.GeprueftAm).After(dateOnly(day)) {
			upTo = append(upTo, r)
		}
	}
	return EvaluateInspections(upTo, pruefintervallMonate, day, warnDays)
}

// loadActiveInspections laedt die nicht stornierten Pruefungen eines
// Assets.
func loadActiveInspections(ctx context.Context, q queryer, assetID uuid.UUID) ([]Inspection, error) {
	rows, err := q.Query(ctx, `SELECT `+inspectionColumns+` FROM asset_inspections i WHERE i.asset_id=$1 AND i.status='active'`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Inspection, 0)
	for rows.Next() {
		i, err := scanInspection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}

// InspectionStandingOf liefert den Pruefstand eines Assets am Tag today.
func (s *Service) InspectionStandingOf(ctx context.Context, assetID uuid.UUID, today time.Time, warnDays int, companyID string) (*InspectionStanding, error) {
	var intervall *int
	if err := s.pg.QueryRow(ctx, `SELECT pruefintervall_monate FROM assets WHERE id=$1 AND company_id=$2`, assetID, companyID).Scan(&intervall); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Asset nicht gefunden")
		}
		return nil, err
	}
	records, err := loadActiveInspections(ctx, s.pg, assetID)
	if err != nil {
		return nil, err
	}
	st := standingAt(records, intervall, today, warnDays)
	return &st, nil
}

// issueHints liefert die Hinweise zu einer Ausgabe. Gesperrt (nicht
// bestanden oder Maengel) ist kein Hinweis, sondern ein Ablehnungsgrund
// (siehe Issue). "Bald faellig" ist keiner: die Ausgabe ist in Ordnung, die
// Liste mahnt an.
func issueHints(st InspectionStanding) []string {
	hints := make([]string, 0, 2)
	switch st.Status {
	case PruefstandUeberfaellig:
		hints = append(hints, fmt.Sprintf("Prüfung überfällig seit %s", st.NaechstePruefungAm.Format("02.01.2006")))
	case PruefstandNieGeprueft:
		hints = append(hints, "Asset wurde noch nie geprüft")
	}
	return hints
}

// AssetDueEntry ist ein offener Punkt der betriebsweiten Liste.
type AssetDueEntry struct {
	AssetID        uuid.UUID `json:"asset_id"`
	Inventarnummer string    `json:"inventarnummer"`
	Bezeichnung    string    `json:"bezeichnung"`
	Kategorie      string    `json:"kategorie"`
	Standort       string    `json:"standort"`
	// Wer das Stueck gerade hat - dort muss es zur Pruefung abgeholt
	// werden. nil, wenn es nicht ausgegeben ist.
	EmployeeID      *uuid.UUID `json:"employee_id,omitempty"`
	MitarbeiterName string     `json:"mitarbeiter_name,omitempty"`
	InspectionStanding
}

// inspectionStandRank: das Dringendste zuerst. Gesperrt vor ueberfaellig:
// ein gesperrtes Stueck darf gar nicht benutzt werden.
func inspectionStandRank(status string) int {
	switch status {
	case PruefstandGesperrt:
		return 0
	case PruefstandUeberfaellig:
		return 1
	case PruefstandNieGeprueft:
		return 2
	case PruefstandFaelligBald:
		return 3
	default:
		return 4
	}
}

// inspectionNeedsAttention: was in die Liste gehoert.
func inspectionNeedsAttention(status string) bool {
	return inspectionStandRank(status) < 4
}

// DueList liefert fuer alle NICHT ausgemusterten Assets des Mandanten, was
// am Tag today gesperrt, ueberfaellig, nie geprueft oder bald faellig ist.
// Gepruefte und nicht pruefpflichtige erscheinen nicht.
//
// Sortierung: das Dringendste zuerst, darin am laengsten ueberfaellig bzw.
// am fruehesten faellig, dann nach Inventarnummer.
//
// Drei Abfragen fuer den ganzen Mandanten, nicht je Asset.
func (s *Service) DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]AssetDueEntry, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}

	type asset struct {
		entry     AssetDueEntry
		intervall *int
	}
	list := make([]asset, 0)
	rows, err := s.pg.Query(ctx, `
		SELECT a.id, a.inventarnummer, a.bezeichnung, a.kategorie, a.standort, a.pruefintervall_monate,
		       x.employee_id, COALESCE(btrim(e.first_name || ' ' || e.last_name), '')
		  FROM assets a
		  LEFT JOIN asset_assignments x ON x.asset_id = a.id AND x.zurueckgegeben_am IS NULL AND x.status = 'active'
		  LEFT JOIN hr_employees e ON e.id = x.employee_id
		 WHERE a.company_id=$1 AND a.ausgemustert_am IS NULL`, companyID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a asset
		if err := rows.Scan(&a.entry.AssetID, &a.entry.Inventarnummer, &a.entry.Bezeichnung, &a.entry.Kategorie, &a.entry.Standort, &a.intervall,
			&a.entry.EmployeeID, &a.entry.MitarbeiterName); err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	records := make(map[uuid.UUID][]Inspection)
	recRows, err := s.pg.Query(ctx, `
		SELECT `+inspectionColumns+`
		  FROM asset_inspections i
		  JOIN assets a ON a.id = i.asset_id
		 WHERE a.company_id=$1 AND a.ausgemustert_am IS NULL AND i.status='active'`, companyID)
	if err != nil {
		return nil, err
	}
	for recRows.Next() {
		i, err := scanInspection(recRows)
		if err != nil {
			recRows.Close()
			return nil, err
		}
		records[i.AssetID] = append(records[i.AssetID], *i)
	}
	recRows.Close()
	if err := recRows.Err(); err != nil {
		return nil, err
	}

	out := make([]AssetDueEntry, 0)
	for _, a := range list {
		st := standingAt(records[a.entry.AssetID], a.intervall, today, warnDays)
		if !inspectionNeedsAttention(st.Status) {
			continue
		}
		e := a.entry
		e.InspectionStanding = st
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ra, rb := inspectionStandRank(a.Status), inspectionStandRank(b.Status); ra != rb {
			return ra < rb
		}
		if a.TageBisFaellig != nil && b.TageBisFaellig != nil && *a.TageBisFaellig != *b.TageBisFaellig {
			return *a.TageBisFaellig < *b.TageBisFaellig
		}
		if a.Inventarnummer != b.Inventarnummer {
			return a.Inventarnummer < b.Inventarnummer
		}
		return a.AssetID.String() < b.AssetID.String()
	})
	return out, nil
}
