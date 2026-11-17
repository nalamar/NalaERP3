package matching

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/quotes/gaeb"
)

// Store liest LV und Katalog fuer einen Lauf aus der Datenbank (ADR 0045,
// Backlog I.5.2a): das LV aus den Importtabellen (Positionen, Gliederung,
// Vorbemerkungen), den Katalog aus den aktiven Leistungen des Mandanten.
type Store struct {
	pg *pgxpool.Pool
}

func NewStore(pg *pgxpool.Pool) *Store { return &Store{pg: pg} }

// ErrImportNotFound: kein GAEB-Importlauf mit dieser ID beim Mandanten.
var ErrImportNotFound = errors.New("Importlauf nicht gefunden")

// LadeLV baut das LV eines Importlaufs wieder auf, wie der Parser es
// geliefert hat. positionen bildet die Ordnungszahl auf die ID der
// Importposition ab (Ziel der Vorschlaege).
func (s *Store) LadeLV(ctx context.Context, importID, companyID string) (lv *gaeb.LV, positionen map[string]string, err error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, nil, errors.New("Mandant erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, nil, ErrImportNotFound
	}
	// Format und Phase braucht der Matcher nicht (am Import nur
	// zusammengesetzt in detected_format).
	lv = &gaeb.LV{}
	err = s.pg.QueryRow(ctx, `
		SELECT i.vorbemerkungen
		FROM quote_imports i JOIN projects p ON p.id = i.project_id
		WHERE i.id=$1 AND i.source_kind='gaeb' AND p.company_id=$2`, importID, companyID).Scan(&lv.Vorbemerkungen)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrImportNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	rows, err := s.pg.Query(ctx, `
		SELECT g.outline_no, g.kind, g.bezeichnung, COALESCE(e.outline_no, '')
		FROM quote_import_groups g LEFT JOIN quote_import_groups e ON e.id = g.parent_group_id
		WHERE g.import_id=$1 ORDER BY g.sort_order, g.id`, importID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var b gaeb.Bereich
		if err := rows.Scan(&b.OZ, &b.Stufe, &b.Bezeichnung, &b.Eltern); err != nil {
			rows.Close()
			return nil, nil, err
		}
		lv.Bereiche = append(lv.Bereiche, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Kurztext fehlt bei Importen vor Migration 116: dann die Beschreibung.
	rows, err = s.pg.Query(ctx, `
		SELECT id::text, position_no, outline_no, CASE WHEN short_text = '' AND long_text = '' THEN description ELSE short_text END,
		       long_text, qty::float8, unit, item_kind, is_lump_sum, base_position_no
		FROM quote_import_items WHERE import_id=$1 ORDER BY sort_order, id`, importID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	positionen = map[string]string{}
	for rows.Next() {
		var id string
		var p gaeb.Position
		if err := rows.Scan(&id, &p.OZ, &p.Bereich, &p.Kurztext, &p.Langtext, &p.Menge, &p.Einheit, &p.Art, &p.Pauschal, &p.Zu); err != nil {
			return nil, nil, err
		}
		if _, doppelt := positionen[p.OZ]; doppelt {
			return nil, nil, fmt.Errorf("Ordnungszahl %s ist doppelt", p.OZ)
		}
		positionen[p.OZ] = id
		lv.Positionen = append(lv.Positionen, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(lv.Positionen) == 0 {
		return nil, nil, errors.New("Importlauf hat keine Positionen")
	}
	return lv, positionen, nil
}

// LadeKatalog liefert die aktiven Leistungen des Mandanten (nach Code) und
// bildet den Code auf die ID der Leistung ab. Leer ist zulaessig; der Lauf
// entscheidet, was dann geschieht.
func (s *Store) LadeKatalog(ctx context.Context, companyID string) (k *Katalog, ids map[string]string, err error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, nil, errors.New("Mandant erforderlich")
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id, code, kurztext, einheit, gewerk, profilserie, verglasung, rc_klasse, u_wert::float8, brandschutzklasse
		FROM leistungen WHERE company_id=$1 AND aktiv ORDER BY code`, companyID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	k = &Katalog{Leistungen: []Leistung{}}
	ids = map[string]string{}
	for rows.Next() {
		var id string
		var l Leistung
		sv := &l.Systemvorgaben
		if err := rows.Scan(&id, &l.Code, &l.Kurztext, &l.Einheit, &l.Gewerk, &sv.Profilserie, &sv.Verglasung, &sv.RCKlasse, &sv.UWert,
			&sv.Brandschutzklasse); err != nil {
			return nil, nil, err
		}
		k.Leistungen = append(k.Leistungen, l)
		ids[l.Code] = id
	}
	return k, ids, rows.Err()
}
