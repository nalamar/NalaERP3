package accounting

// E-Rechnung Eingang, Lieferanten-Zuordnungsvorschlag (ADR 0023,
// Backlog E.5.5).
//
// Liefert ausdruecklich nur einen VORSCHLAG. Laut ADR 0023 wird beim
// Eingang nichts automatisch angelegt und nichts automatisch zugeordnet:
// invoices_in.supplier_id ist ein Pflicht-Fremdschluessel auf contacts,
// und eine per Textaehnlichkeit geratene Zuordnung in einem
// buchungsrelevanten Kontext waere nicht vertretbar. Die Uebernahme
// bestaetigt ein Mensch (Backlog E.6).

import (
	"context"
	"fmt"
	"strings"
)

// Wie der Vorschlag zustande kam - fuer die Anzeige wichtiger als der
// Treffer selbst, weil daran haengt, wie sehr man ihm trauen darf.
const (
	SupplierMatchNone  = "kein_treffer"
	SupplierMatchVatID = "ust_id"
	SupplierMatchName  = "name"
	SupplierMatchAmbig = "mehrdeutig"
)

// SupplierCandidate ist ein moeglicher Lieferant aus den Stammdaten.
type SupplierCandidate struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	VatID string `json:"ust_id"`
}

// SupplierSuggestion ist das Ergebnis der Zuordnung.
//
// Eindeutig ist bewusst ein eigenes Feld und nicht aus len(Kandidaten)
// ableitbar gedacht: die anzeigende Seite soll nicht selbst entscheiden
// muessen, ab wann ein Treffer belastbar ist.
type SupplierSuggestion struct {
	Art        string              `json:"art"`
	Eindeutig  bool                `json:"eindeutig"`
	Vorschlag  *SupplierCandidate  `json:"vorschlag,omitempty"`
	Kandidaten []SupplierCandidate `json:"kandidaten,omitempty"`
	Hinweis    string              `json:"hinweis,omitempty"`
}

// SuggestSupplier sucht zum Verkaeufer einer eingegangenen Rechnung
// passende Kontakte.
//
// Reihenfolge der Kriterien (ADR 0023): zuerst die USt-IdNr., weil sie
// eine eindeutige Kennung ist; nur wenn darueber nichts zu finden ist,
// der Name - und ein Namenstreffer wird NIE als eindeutig ausgewiesen,
// auch wenn es nur einer ist. Gleiche Firmennamen sind haeufig, und ein
// falsch zugeordneter Lieferant faellt spaeter kaum auf.
func (s *APService) SuggestSupplier(ctx context.Context, seller ParsedParty, companyID string) (*SupplierSuggestion, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, fmt.Errorf("Mandant erforderlich")
	}

	if vat := normalizeVatID(seller.VatID); vat != "" {
		kandidaten, err := s.findSuppliersByVatID(ctx, vat, companyID)
		if err != nil {
			return nil, err
		}
		switch len(kandidaten) {
		case 0:
			// Weiter zum Namensabgleich - eine unbekannte USt-IdNr.
			// bedeutet nur, dass der Lieferant so noch nicht gepflegt ist.
		case 1:
			return &SupplierSuggestion{
				Art:        SupplierMatchVatID,
				Eindeutig:  true,
				Vorschlag:  &kandidaten[0],
				Kandidaten: kandidaten,
			}, nil
		default:
			return &SupplierSuggestion{
				Art:        SupplierMatchAmbig,
				Eindeutig:  false,
				Kandidaten: kandidaten,
				Hinweis: fmt.Sprintf(
					"%d Kontakte tragen dieselbe USt-IdNr. %s - bitte den richtigen Lieferanten auswählen.",
					len(kandidaten), seller.VatID),
			}, nil
		}
	}

	name := strings.TrimSpace(seller.Name)
	if name == "" {
		return &SupplierSuggestion{
			Art:       SupplierMatchNone,
			Eindeutig: false,
			Hinweis:   "Die Rechnung nennt weder eine USt-IdNr. noch einen Verkäufernamen - eine Zuordnung ist nicht möglich.",
		}, nil
	}

	kandidaten, err := s.findSuppliersByName(ctx, name, companyID)
	if err != nil {
		return nil, err
	}
	if len(kandidaten) == 0 {
		return &SupplierSuggestion{
			Art:       SupplierMatchNone,
			Eindeutig: false,
			Hinweis: fmt.Sprintf(
				"Zu %q wurde kein Lieferant gefunden - der Kontakt muss vermutlich zuerst angelegt werden.", name),
		}, nil
	}

	// Bewusst NIE eindeutig: ein Namenstreffer ist ein Anhaltspunkt, kein
	// Beweis. Der Vorschlag wird dennoch benannt, damit die Anzeige einen
	// Startpunkt hat.
	sug := &SupplierSuggestion{
		Art:        SupplierMatchName,
		Eindeutig:  false,
		Vorschlag:  &kandidaten[0],
		Kandidaten: kandidaten,
	}
	if len(kandidaten) == 1 {
		sug.Hinweis = "Treffer nur über den Namen (die Rechnung nennt keine bekannte USt-IdNr.) - bitte prüfen."
	} else {
		sug.Hinweis = fmt.Sprintf("%d Kontakte passen dem Namen nach - bitte den richtigen Lieferanten auswählen.", len(kandidaten))
	}
	return sug, nil
}

func (s *APService) findSuppliersByVatID(ctx context.Context, vat, companyID string) ([]SupplierCandidate, error) {
	// Vergleich normalisiert: USt-IdNrn. werden in der Praxis mal mit,
	// mal ohne Leerzeichen erfasst ("DE 123456789" vs. "DE123456789").
	rows, err := s.pg.Query(ctx, `
        SELECT id, name, COALESCE(vat_id,'')
          FROM contacts
         WHERE company_id = $1
           AND aktiv
           AND rolle IN ('supplier','both')
           AND upper(replace(replace(COALESCE(vat_id,''), ' ', ''), '-', '')) = $2
         ORDER BY name, id
    `, companyID, vat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSupplierCandidates(rows)
}

func (s *APService) findSuppliersByName(ctx context.Context, name, companyID string) ([]SupplierCandidate, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT id, name, COALESCE(vat_id,'')
          FROM contacts
         WHERE company_id = $1
           AND aktiv
           AND rolle IN ('supplier','both')
           AND lower(btrim(name)) = lower(btrim($2))
         ORDER BY name, id
    `, companyID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSupplierCandidates(rows)
}

// rowScanner deckt genau das ab, was hier von pgx.Rows gebraucht wird.
type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanSupplierCandidates(rows rowScanner) ([]SupplierCandidate, error) {
	var out []SupplierCandidate
	for rows.Next() {
		var c SupplierCandidate
		if err := rows.Scan(&c.ID, &c.Name, &c.VatID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// normalizeVatID macht USt-IdNrn. vergleichbar (Grossschreibung, ohne
// Leerzeichen und Bindestriche).
func normalizeVatID(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "")
	v = strings.ReplaceAll(v, "-", "")
	return v
}
