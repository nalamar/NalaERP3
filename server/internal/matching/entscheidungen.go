package matching

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Pruefliste und Entscheidungen (ADR 0045 Frage 5, Backlog I.5.2c): jeder
// Vorschlag bleibt offen, bis ein Mensch ihn bestaetigt, aendert (andere
// oder erstmals eine Leistung) oder verwirft; ueber der Schwelle gibt es die
// Sammelbestaetigung als bewusste Aktion. Entscheiden geht nur, solange der
// Importlauf verarbeitet und nicht freigegeben ist; eine Entscheidung darf
// bis dahin geaendert werden.

const (
	AktionBestaetigen = "bestaetigen"
	AktionAendern     = "aendern"
	AktionVerwerfen   = "verwerfen"
)

// PruefZeile ist eine Position des Importlaufs mit ihrem Vorschlag (ohne
// Vorschlag: Quelle leer).
type PruefZeile struct {
	ItemID                string          `json:"item_id"`
	PositionNo            string          `json:"position_no"`
	Kurztext              string          `json:"kurztext"`
	Menge                 float64         `json:"menge"`
	Einheit               string          `json:"einheit"`
	LeistungID            *string         `json:"leistung_id"`
	Code                  string          `json:"code"`
	Score                 float64         `json:"score"`
	Begruendung           string          `json:"begruendung"`
	Systemvorgaben        json.RawMessage `json:"systemvorgaben"`
	Quelle                string          `json:"quelle"`
	ProfilName            string          `json:"profil_name"`
	Modell                string          `json:"modell"`
	Schwelle              float64         `json:"schwelle"`
	UeberSchwelle         bool            `json:"ueber_schwelle"`
	Entscheidung          string          `json:"entscheidung"`
	EntschiedenLeistungID *string         `json:"entschieden_leistung_id"`
	EntschiedenCode       string          `json:"entschieden_code"`
	EntschiedenVonUserID  *string         `json:"entschieden_von_user_id"`
	EntschiedenAm         *time.Time      `json:"entschieden_at"`
}

// EntscheidungEingabe: LeistungID nur bei "aendern".
type EntscheidungEingabe struct {
	Aktion     string `json:"aktion"`
	LeistungID string `json:"leistung_id"`
}

// ErrItemNotFound: keine Position mit dieser ID im Importlauf.
var ErrItemNotFound = errors.New("Importposition nicht gefunden")

const pruefSelect = `SELECT i.id::text, i.position_no,
	CASE WHEN i.short_text = '' AND i.long_text = '' THEN i.description ELSE i.short_text END, i.qty::float8, i.unit,
	m.leistung_id, COALESCE(m.code, ''), COALESCE(m.score, 0)::float8, COALESCE(m.begruendung, ''), COALESCE(m.systemvorgaben, '{}'),
	COALESCE(m.quelle, ''), COALESCE(m.profil_name, ''), COALESCE(m.modell, ''), COALESCE(m.schwelle, 0)::float8,
	COALESCE(m.code <> '' AND m.score >= m.schwelle, false), COALESCE(m.entscheidung, ''), m.entschieden_leistung_id,
	COALESCE(m.entschieden_code, ''), m.entschieden_von_user_id, m.entschieden_at
	FROM quote_import_items i LEFT JOIN quote_import_item_matches m ON m.quote_import_item_id = i.id
	WHERE i.import_id=$1`

func scanPruefZeile(row pgx.Row) (*PruefZeile, error) {
	var z PruefZeile
	err := row.Scan(&z.ItemID, &z.PositionNo, &z.Kurztext, &z.Menge, &z.Einheit, &z.LeistungID, &z.Code, &z.Score, &z.Begruendung,
		&z.Systemvorgaben, &z.Quelle, &z.ProfilName, &z.Modell, &z.Schwelle, &z.UeberSchwelle, &z.Entscheidung, &z.EntschiedenLeistungID,
		&z.EntschiedenCode, &z.EntschiedenVonUserID, &z.EntschiedenAm)
	return &z, err
}

// Pruefliste liefert alle Positionen des Importlaufs in LV-Reihenfolge mit
// ihrem Vorschlag; die Reihenfolge der Pruefung waehlt der Client.
func (s *LaufService) Pruefliste(ctx context.Context, importID, companyID string) ([]PruefZeile, error) {
	if _, _, err := s.store.importStatus(ctx, s.store.pg.QueryRow, importID, companyID, ""); err != nil {
		return nil, err
	}
	rows, err := s.store.pg.Query(ctx, pruefSelect+` ORDER BY i.sort_order, i.id`, importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PruefZeile{}
	for rows.Next() {
		z, err := scanPruefZeile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *z)
	}
	return out, rows.Err()
}

// beginEntscheidung sperrt den Importlauf (gegen gleichzeitige Freigabe) und
// prueft, dass entschieden werden darf.
func (s *LaufService) beginEntscheidung(ctx context.Context, importID, companyID string) (pgx.Tx, error) {
	tx, err := s.store.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	_, status, err := s.store.importStatus(ctx, tx.QueryRow, importID, companyID, " FOR UPDATE OF i")
	if err == nil && status != "parsed" {
		err = errors.New("Entscheiden im aktuellen Status nicht erlaubt (nur verarbeitete, noch nicht freigegebene Importläufe)")
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

// Entscheiden haelt die Entscheidung zu einer Position fest.
func (s *LaufService) Entscheiden(ctx context.Context, importID, itemID string, in EntscheidungEingabe, actorUserID, companyID string) (*PruefZeile, error) {
	in.Aktion = strings.TrimSpace(in.Aktion)
	in.LeistungID = strings.TrimSpace(in.LeistungID)
	switch {
	case in.Aktion != AktionBestaetigen && in.Aktion != AktionAendern && in.Aktion != AktionVerwerfen:
		return nil, errors.New("Aktion ungültig (bestaetigen, aendern oder verwerfen)")
	case (in.Aktion == AktionAendern) != (in.LeistungID != ""):
		return nil, errors.New("Leistung ungültig: nur und immer beim Ändern angeben")
	}
	tx, err := s.beginEntscheidung(ctx, importID, companyID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if !istUUID(itemID) {
		return nil, ErrItemNotFound
	}
	var vorhanden, mitLeistung bool
	err = tx.QueryRow(ctx, `SELECT m.quote_import_item_id IS NOT NULL, m.leistung_id IS NOT NULL
		FROM quote_import_items i LEFT JOIN quote_import_item_matches m ON m.quote_import_item_id = i.id
		WHERE i.import_id=$1 AND i.id=$2 FOR UPDATE OF i`, importID, itemID).Scan(&vorhanden, &mitLeistung)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrItemNotFound
	}
	if err != nil {
		return nil, err
	}
	if !vorhanden {
		return nil, errors.New("Für diese Position fehlt ein Vorschlag - zuerst die Zuordnung starten")
	}
	var actor *string
	if strings.TrimSpace(actorUserID) != "" {
		actor = &actorUserID
	}
	switch in.Aktion {
	case AktionBestaetigen:
		// ohne Code gibt es nie eine Leistung
		if !mitLeistung {
			return nil, errors.New("Bestätigen nicht möglich: es fehlt eine vorgeschlagene, vorhandene Leistung - sonst ändern")
		}
		_, err = tx.Exec(ctx, `UPDATE quote_import_item_matches SET entscheidung='bestaetigt', entschieden_leistung_id=NULL,
			entschieden_code='', entschieden_von_user_id=$2, entschieden_at=now() WHERE quote_import_item_id=$1`, itemID, actor)
	case AktionVerwerfen:
		_, err = tx.Exec(ctx, `UPDATE quote_import_item_matches SET entscheidung='verworfen', entschieden_leistung_id=NULL,
			entschieden_code='', entschieden_von_user_id=$2, entschieden_at=now() WHERE quote_import_item_id=$1`, itemID, actor)
	case AktionAendern:
		var code string
		err = tx.QueryRow(ctx, `SELECT code FROM leistungen WHERE id=$1 AND company_id=$2 AND aktiv`, in.LeistungID, companyID).Scan(&code)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Leistung nicht gefunden oder abgeschaltet")
		}
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `UPDATE quote_import_item_matches SET entscheidung='geaendert', entschieden_leistung_id=$2,
			entschieden_code=$3, entschieden_von_user_id=$4, entschieden_at=now() WHERE quote_import_item_id=$1`, itemID, in.LeistungID, code, actor)
	}
	if err != nil {
		return nil, err
	}
	z, err := scanPruefZeile(tx.QueryRow(ctx, pruefSelect+` AND i.id=$2`, importID, itemID))
	if err != nil {
		return nil, err
	}
	return z, tx.Commit(ctx)
}

// SammelBestaetigen bestaetigt alle offenen Vorschlaege mit vorhandener
// Leistung, deren Score die Schwelle des Vorschlags erreicht, und liefert
// ihre Anzahl.
func (s *LaufService) SammelBestaetigen(ctx context.Context, importID, actorUserID, companyID string) (int64, error) {
	tx, err := s.beginEntscheidung(ctx, importID, companyID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var actor *string
	if strings.TrimSpace(actorUserID) != "" {
		actor = &actorUserID
	}
	tag, err := tx.Exec(ctx, `UPDATE quote_import_item_matches SET entscheidung='bestaetigt', entschieden_von_user_id=$2, entschieden_at=now()
		WHERE import_id=$1 AND entscheidung='offen' AND code <> '' AND leistung_id IS NOT NULL AND score >= schwelle`, importID, actor)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), tx.Commit(ctx)
}
