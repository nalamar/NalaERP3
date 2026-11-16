// Package leistungen fuehrt den Leistungskatalog je Mandant (ADR 0044
// Frage 1, Backlog I.4.4a): Ziel des Matchings von LV-Positionen. Preise
// stehen nicht im Katalog.
package leistungen

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxCode         = 50
	maxKurztext     = 500
	maxLangtext     = 20000
	maxEinheit      = 20
	maxGewerk       = 100
	maxMerkmal      = 200
	MaxImportZeilen = 5000
	standardLimit   = 200
	maxLimit        = 1000
)

var reRC = regexp.MustCompile(`^RC[1-6]$`)

// Leistung ist ein Eintrag im Katalog.
type Leistung struct {
	ID                string    `json:"id"`
	Code              string    `json:"code"`
	Kurztext          string    `json:"kurztext"`
	Langtext          string    `json:"langtext"`
	Einheit           string    `json:"einheit"`
	Gewerk            string    `json:"gewerk"`
	Profilserie       string    `json:"profilserie"`
	Verglasung        string    `json:"verglasung"`
	RCKlasse          string    `json:"rc_klasse"`
	UWert             *float64  `json:"u_wert"`
	Brandschutzklasse string    `json:"brandschutzklasse"`
	Aktiv             bool      `json:"aktiv"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	UpdatedByUserID   *string   `json:"updated_by_user_id"`
}

// Eingabe ist der vollstaendige Stand einer Leistung (Anlegen, Aendern,
// Import). Aktiv nil = aktiv (beim Anlegen) bzw. unveraendert (beim Import
// einer vorhandenen Leistung).
type Eingabe struct {
	Code              string   `json:"code"`
	Kurztext          string   `json:"kurztext"`
	Langtext          string   `json:"langtext"`
	Einheit           string   `json:"einheit"`
	Gewerk            string   `json:"gewerk"`
	Profilserie       string   `json:"profilserie"`
	Verglasung        string   `json:"verglasung"`
	RCKlasse          string   `json:"rc_klasse"`
	UWert             *float64 `json:"u_wert"`
	Brandschutzklasse string   `json:"brandschutzklasse"`
	Aktiv             *bool    `json:"aktiv"`
}

// Filter fuer die Liste: Q sucht in Code und Kurztext; NurAktive blendet
// abgeschaltete aus.
type Filter struct {
	Q         string
	NurAktive bool
	Limit     int
	Offset    int
}

// Normalisieren entfernt Leerraum am Rand; die RC-Klasse wird gross und ohne
// Leerzeichen/Bindestrich geschrieben ("rc 2" -> "RC2").
func Normalisieren(in Eingabe) Eingabe {
	in.Code = strings.TrimSpace(in.Code)
	in.Kurztext = strings.TrimSpace(in.Kurztext)
	in.Langtext = strings.TrimSpace(in.Langtext)
	in.Einheit = strings.TrimSpace(in.Einheit)
	in.Gewerk = strings.TrimSpace(in.Gewerk)
	in.Profilserie = strings.TrimSpace(in.Profilserie)
	in.Verglasung = strings.TrimSpace(in.Verglasung)
	in.RCKlasse = strings.NewReplacer(" ", "", "-", "").Replace(strings.ToUpper(strings.TrimSpace(in.RCKlasse)))
	in.Brandschutzklasse = strings.TrimSpace(in.Brandschutzklasse)
	return in
}

// Pruefen meldet den ersten Fehler einer normalisierten Eingabe.
func Pruefen(in Eingabe) error {
	laenger := func(s string, n int) bool { return utf8.RuneCountInString(s) > n }
	switch {
	case in.Code == "":
		return errors.New("Code erforderlich")
	case laenger(in.Code, maxCode) || strings.ContainsAny(in.Code, " \t\r\n"):
		return fmt.Errorf("Code ungültig (ohne Leerzeichen, höchstens %d Zeichen)", maxCode)
	case in.Kurztext == "":
		return errors.New("Kurztext erforderlich")
	case laenger(in.Kurztext, maxKurztext):
		return fmt.Errorf("Kurztext ungültig (höchstens %d Zeichen)", maxKurztext)
	case laenger(in.Langtext, maxLangtext):
		return fmt.Errorf("Langtext ungültig (höchstens %d Zeichen)", maxLangtext)
	case in.Einheit == "":
		return errors.New("Einheit erforderlich")
	case laenger(in.Einheit, maxEinheit):
		return fmt.Errorf("Einheit ungültig (höchstens %d Zeichen)", maxEinheit)
	case laenger(in.Gewerk, maxGewerk):
		return fmt.Errorf("Gewerk ungültig (höchstens %d Zeichen)", maxGewerk)
	case laenger(in.Profilserie, maxMerkmal), laenger(in.Verglasung, maxMerkmal), laenger(in.Brandschutzklasse, maxMerkmal):
		return fmt.Errorf("Systemvorgabe ungültig (höchstens %d Zeichen)", maxMerkmal)
	case in.RCKlasse != "" && !reRC.MatchString(in.RCKlasse):
		return errors.New("RC-Klasse ungültig (RC1 bis RC6)")
	case in.UWert != nil && (*in.UWert <= 0 || *in.UWert > 10):
		return errors.New("U-Wert ungültig (über 0 bis 10)")
	}
	return nil
}

// Service liest und schreibt den Katalog.
type Service struct{ pg *pgxpool.Pool }

func NewService(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

const spalten = `id, code, kurztext, langtext, einheit, gewerk, profilserie, verglasung, rc_klasse,
    u_wert::float8, brandschutzklasse, aktiv, created_at, updated_at, updated_by_user_id`

func scan(row pgx.Row) (*Leistung, error) {
	var l Leistung
	err := row.Scan(&l.ID, &l.Code, &l.Kurztext, &l.Langtext, &l.Einheit, &l.Gewerk, &l.Profilserie, &l.Verglasung,
		&l.RCKlasse, &l.UWert, &l.Brandschutzklasse, &l.Aktiv, &l.CreatedAt, &l.UpdatedAt, &l.UpdatedByUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Leistung nicht gefunden")
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func codeVergeben(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "leistungen_code_unique"
}

func actor(id string) *string {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	return &id
}

func mandant(companyID string) error {
	if strings.TrimSpace(companyID) == "" {
		return errors.New("Mandant erforderlich")
	}
	return nil
}

// Anlegen legt eine Leistung an; der Code ist je Mandant eindeutig.
func (s *Service) Anlegen(ctx context.Context, in Eingabe, actorUserID, companyID string) (*Leistung, error) {
	if err := mandant(companyID); err != nil {
		return nil, err
	}
	in = Normalisieren(in)
	if err := Pruefen(in); err != nil {
		return nil, err
	}
	aktiv := in.Aktiv == nil || *in.Aktiv
	l, err := scan(s.pg.QueryRow(ctx, `
        INSERT INTO leistungen (id, company_id, code, kurztext, langtext, einheit, gewerk, profilserie, verglasung,
            rc_klasse, u_wert, brandschutzklasse, aktiv, updated_by_user_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
        RETURNING `+spalten, uuid.NewString(), companyID, in.Code, in.Kurztext, in.Langtext, in.Einheit, in.Gewerk,
		in.Profilserie, in.Verglasung, in.RCKlasse, in.UWert, in.Brandschutzklasse, aktiv, actor(actorUserID)))
	if codeVergeben(err) {
		return nil, fmt.Errorf("Code %q bereits vorhanden", in.Code)
	}
	return l, err
}

// Aendern ersetzt den Stand einer Leistung des Mandanten. Aktiv nil laesst
// den Schalter unveraendert.
func (s *Service) Aendern(ctx context.Context, id string, in Eingabe, actorUserID, companyID string) (*Leistung, error) {
	if err := mandant(companyID); err != nil {
		return nil, err
	}
	in = Normalisieren(in)
	if err := Pruefen(in); err != nil {
		return nil, err
	}
	l, err := scan(s.pg.QueryRow(ctx, `
        UPDATE leistungen SET code=$3, kurztext=$4, langtext=$5, einheit=$6, gewerk=$7, profilserie=$8, verglasung=$9,
            rc_klasse=$10, u_wert=$11, brandschutzklasse=$12, aktiv=COALESCE($13, aktiv), updated_by_user_id=$14, updated_at=now()
        WHERE id=$1 AND company_id=$2
        RETURNING `+spalten, strings.TrimSpace(id), companyID, in.Code, in.Kurztext, in.Langtext, in.Einheit, in.Gewerk,
		in.Profilserie, in.Verglasung, in.RCKlasse, in.UWert, in.Brandschutzklasse, in.Aktiv, actor(actorUserID)))
	if codeVergeben(err) {
		return nil, fmt.Errorf("Code %q bereits vorhanden", in.Code)
	}
	return l, err
}

// Lesen liefert eine Leistung des Mandanten.
func (s *Service) Lesen(ctx context.Context, id, companyID string) (*Leistung, error) {
	if err := mandant(companyID); err != nil {
		return nil, err
	}
	return scan(s.pg.QueryRow(ctx, `SELECT `+spalten+` FROM leistungen WHERE id=$1 AND company_id=$2`, strings.TrimSpace(id), companyID))
}

// Liste liefert die Leistungen des Mandanten nach Code sortiert.
func (s *Service) Liste(ctx context.Context, f Filter, companyID string) ([]Leistung, error) {
	if err := mandant(companyID); err != nil {
		return nil, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = standardLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	q := strings.TrimSpace(f.Q)
	rows, err := s.pg.Query(ctx, `SELECT `+spalten+` FROM leistungen
        WHERE company_id=$1 AND (NOT $2 OR aktiv)
          AND ($3 = '' OR code ILIKE '%' || $3 || '%' OR kurztext ILIKE '%' || $3 || '%')
        ORDER BY code LIMIT $4 OFFSET $5`, companyID, f.NurAktive, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Leistung{}
	for rows.Next() {
		l, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

// ImportErgebnis zaehlt angelegte und geaenderte Leistungen.
type ImportErgebnis struct {
	Neu       int `json:"neu"`
	Geaendert int `json:"geaendert"`
}

// Importieren legt Leistungen an oder aendert sie ueber den Code - ganz
// oder gar nicht. Alle Zeilen werden vorher geprueft; die Meldung nennt die
// Zeile: nummern[i] fuer zeilen[i] (z. B. Dateizeile aus LeseCSV), ohne
// nummern ab 1. Fehlt "aktiv", bleibt der Schalter vorhandener Leistungen
// unveraendert, neue sind aktiv. Leistungen, die nicht in der Datei stehen,
// bleiben unberuehrt.
func (s *Service) Importieren(ctx context.Context, zeilen []Eingabe, nummern []int, actorUserID, companyID string) (*ImportErgebnis, error) {
	if err := mandant(companyID); err != nil {
		return nil, err
	}
	if len(zeilen) == 0 {
		return nil, errors.New("Import ungültig: keine Leistungen")
	}
	if len(zeilen) > MaxImportZeilen {
		return nil, fmt.Errorf("Import ungültig: höchstens %d Leistungen je Datei", MaxImportZeilen)
	}
	if nummern != nil && len(nummern) != len(zeilen) {
		return nil, errors.New("Import ungültig: Zeilennummern passen nicht zu den Leistungen")
	}
	nr := func(i int) int {
		if nummern == nil {
			return i + 1
		}
		return nummern[i]
	}
	gesehen := make(map[string]int, len(zeilen))
	for i := range zeilen {
		zeilen[i] = Normalisieren(zeilen[i])
		if err := Pruefen(zeilen[i]); err != nil {
			return nil, fmt.Errorf("Zeile %d: %w", nr(i), err)
		}
		if vorher, ok := gesehen[zeilen[i].Code]; ok {
			return nil, fmt.Errorf("Zeile %d: Code %q ungültig - doppelt (schon in Zeile %d)", nr(i), zeilen[i].Code, vorher)
		}
		gesehen[zeilen[i].Code] = nr(i)
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	erg := &ImportErgebnis{}
	for _, in := range zeilen {
		var neu bool
		err := tx.QueryRow(ctx, `
            INSERT INTO leistungen (id, company_id, code, kurztext, langtext, einheit, gewerk, profilserie, verglasung,
                rc_klasse, u_wert, brandschutzklasse, aktiv, updated_by_user_id)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,COALESCE($13, true),$14)
            ON CONFLICT (company_id, code) DO UPDATE SET kurztext=EXCLUDED.kurztext, langtext=EXCLUDED.langtext,
                einheit=EXCLUDED.einheit, gewerk=EXCLUDED.gewerk, profilserie=EXCLUDED.profilserie,
                verglasung=EXCLUDED.verglasung, rc_klasse=EXCLUDED.rc_klasse, u_wert=EXCLUDED.u_wert,
                brandschutzklasse=EXCLUDED.brandschutzklasse, aktiv=COALESCE($13, leistungen.aktiv),
                updated_by_user_id=EXCLUDED.updated_by_user_id, updated_at=now()
            RETURNING (xmax = 0)`, uuid.NewString(), companyID, in.Code, in.Kurztext, in.Langtext, in.Einheit, in.Gewerk,
			in.Profilserie, in.Verglasung, in.RCKlasse, in.UWert, in.Brandschutzklasse, in.Aktiv, actor(actorUserID)).Scan(&neu)
		if err != nil {
			return nil, err
		}
		if neu {
			erg.Neu++
		} else {
			erg.Geaendert++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return erg, nil
}
