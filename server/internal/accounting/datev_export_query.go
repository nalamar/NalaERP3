package accounting

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatevExportService lädt Buchungen aus journal_entries/journal_lines für
// den DATEV-Export (ADR 0021, Backlog E.3.3.2) und löst dabei das
// Konto/Gegenkonto-Paar je Buchungssatz auf. Reine Lesefunktion, keine
// Schreiblogik.
type DatevExportService struct{ pg *pgxpool.Pool }

func NewDatevExportService(pg *pgxpool.Pool) *DatevExportService {
	return &DatevExportService{pg: pg}
}

// datevJournalLine ist eine einzelne, bereits um die Kostenstellen-Code-
// Auflösung angereicherte Buchungszeile innerhalb einer Buchung.
type datevJournalLine struct {
	accountCode string
	debit       float64
	credit      float64
	memo        string
	kost1       string // cost_centers.code, leer wenn keine Kostenstelle zugeordnet
}

type datevJournalEntry struct {
	id          string
	entryDate   time.Time
	description string
	sourceID    string
	lines       []datevJournalLine
}

// LoadBookingRows lädt alle Buchungen von `von` bis `bis` (beide
// einschließlich) für den Mandanten und wandelt sie in DatevBookingRow-
// Zeilen um (siehe ADR 0021, "Gegenkonto bei Mehrzeilern"). Buchungen mit
// mehreren Zeilen auf BEIDEN Seiten (Soll UND Haben) haben kein eindeutig
// ableitbares Gegenkonto - der gesamte Export schlägt in diesem Fall mit
// einer Fehlermeldung fehl, die die betroffene Buchung benennt, statt eine
// unvollständige Datei stillschweigend zu erzeugen (GoBD-Vollständigkeits-
// gebot: ein fehlender Hinweis auf eine übersprungene Buchung wäre leichter
// zu übersehen als ein fehlgeschlagener Export).
func (s *DatevExportService) LoadBookingRows(ctx context.Context, companyID string, von, bis time.Time) ([]DatevBookingRow, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT je.id, je.entry_date, je.description, je.source_id,
               jl.account_code, jl.debit, jl.credit, jl.memo, COALESCE(cc.code, '')
        FROM journal_entries je
        JOIN journal_lines jl ON jl.entry_id = je.id
        LEFT JOIN cost_centers cc ON cc.id = jl.kostenstelle_id
        WHERE je.company_id = $1 AND je.entry_date >= $2 AND je.entry_date <= $3
        ORDER BY je.entry_date, je.id, jl.id
    `, companyID, von, bis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*datevJournalEntry
	byID := make(map[string]*datevJournalEntry)

	for rows.Next() {
		var (
			entryID   string
			entryDate time.Time
			desc      string
			sourceID  string
			line      datevJournalLine
		)
		if err := rows.Scan(&entryID, &entryDate, &desc, &sourceID,
			&line.accountCode, &line.debit, &line.credit, &line.memo, &line.kost1); err != nil {
			return nil, err
		}
		entry, ok := byID[entryID]
		if !ok {
			entry = &datevJournalEntry{id: entryID, entryDate: entryDate, description: desc, sourceID: sourceID}
			byID[entryID] = entry
			entries = append(entries, entry)
		}
		entry.lines = append(entry.lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []DatevBookingRow
	for _, entry := range entries {
		bookingRows, err := datevResolveEntryRows(entry.lines)
		if err != nil {
			return nil, fmt.Errorf("DATEV-Export: Buchung %s vom %s: %w", entry.id, entry.entryDate.Format("2006-01-02"), err)
		}
		for _, row := range bookingRows {
			row.Belegdatum = entry.entryDate
			row.Belegfeld1 = entry.sourceID
			if row.Buchungstext == "" {
				row.Buchungstext = entry.description
			}
			out = append(out, row)
		}
	}
	return out, nil
}

// datevResolveEntryRows bildet die Zeilen EINER Buchung auf DatevBookingRow
// ab. Konto/Gegenkonto sind nur eindeutig ableitbar, wenn mindestens eine
// der beiden Seiten (Soll/Haben) aus genau einer Zeile besteht (1:1- und
// 1:N/N:1-Splittbuchungen, siehe ADR 0021) - andernfalls wird ein Fehler
// zurückgegeben. Zeilen mit debit=0 UND credit=0 tragen zu keiner Seite bei
// und werden ignoriert (können bei einer im Übrigen ausgeglichenen Buchung
// vorkommen, sind aber niemals als eigene Buchungssatz-Zeile exportierbar,
// da Umsatz>0 vorausgesetzt wird).
func datevResolveEntryRows(lines []datevJournalLine) ([]DatevBookingRow, error) {
	var soll, haben []datevJournalLine
	for _, l := range lines {
		switch {
		case l.debit > 0:
			soll = append(soll, l)
		case l.credit > 0:
			haben = append(haben, l)
		}
	}
	if len(soll) == 0 || len(haben) == 0 {
		return nil, nil
	}

	switch {
	case len(soll) == 1:
		// Deckt auch den Normalfall 1:1 ab - dann wird per Konvention aus
		// Haben-Sicht gebucht (Konto=Haben-Konto). Beide Varianten sind
		// buchhalterisch gleichwertig, eine feste Konvention macht das
		// Ergebnis deterministisch.
		gegenkonto := soll[0].accountCode
		out := make([]DatevBookingRow, 0, len(haben))
		for _, h := range haben {
			out = append(out, DatevBookingRow{
				Umsatz:       h.credit,
				SollHaben:    "H",
				Konto:        h.accountCode,
				Gegenkonto:   gegenkonto,
				Buchungstext: h.memo,
				Kost1:        h.kost1,
			})
		}
		return out, nil
	case len(haben) == 1:
		gegenkonto := haben[0].accountCode
		out := make([]DatevBookingRow, 0, len(soll))
		for _, sLine := range soll {
			out = append(out, DatevBookingRow{
				Umsatz:       sLine.debit,
				SollHaben:    "S",
				Konto:        sLine.accountCode,
				Gegenkonto:   gegenkonto,
				Buchungstext: sLine.memo,
				Kost1:        sLine.kost1,
			})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("mehrere Zeilen auf beiden Seiten (Soll: %d, Haben: %d) - kein eindeutiges Gegenkonto ableitbar", len(soll), len(haben))
	}
}
