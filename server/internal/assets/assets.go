package assets

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/hr"
)

// Bestand der Betriebsmittel (ADR 0028, Backlog F.3.3): Einzelstuecke mit
// Inventarnummer je Mandant - Werkzeug, Leitern, Messmittel, IT, PSA wie
// der Auffanggurt. Verbrauchs-PSA gehoert nicht hierher (Backlog F.12).
//
// Ein Asset wird nie geloescht, sondern ausgemustert. Einen gespeicherten
// Status gibt es nicht (Migration 089): "ausgegeben" folgt aus den
// Ausgaben (F.3.4), "gesperrt"/"ueberfaellig" aus den Pruefungen (F.3.5).

const (
	KategorieWerkzeug        = "werkzeug"
	KategorieElektrowerkzeug = "elektrowerkzeug"
	KategorieMessmittel      = "messmittel"
	KategorieLeiter          = "leiter"
	KategoriePSA             = "psa"
	KategorieIT              = "it"
	KategorieMaschine        = "maschine"
	KategorieSonstiges       = "sonstiges"
)

var kategorien = map[string]bool{
	KategorieWerkzeug:        true,
	KategorieElektrowerkzeug: true,
	KategorieMessmittel:      true,
	KategorieLeiter:          true,
	KategoriePSA:             true,
	KategorieIT:              true,
	KategorieMaschine:        true,
	KategorieSonstiges:       true,
}

// Inventarnummern stehen auf einem Etikett am Stueck; laenger als das ist
// ein Eingabefehler.
const maxInventarnummerLen = 64

// Name des UNIQUE-Constraints aus Migration 089, dessen Verletzung in eine
// fachliche Meldung uebersetzt wird. Uebersetzung statt Vorabpruefung:
// richtig auch bei zwei gleichzeitigen Aufrufen.
const inventarnummerConstraint = "assets_company_inventarnummer_key"

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

type Asset struct {
	ID                uuid.UUID  `json:"id"`
	Inventarnummer    string     `json:"inventarnummer"`
	Bezeichnung       string     `json:"bezeichnung"`
	Kategorie         string     `json:"kategorie"`
	Seriennummer      string     `json:"seriennummer"`
	Hersteller        string     `json:"hersteller"`
	Modell            string     `json:"modell"`
	Standort          string     `json:"standort"`
	Anschaffungsdatum *time.Time `json:"anschaffungsdatum"`
	Anschaffungspreis *float64   `json:"anschaffungspreis"`
	KostenstelleID    *string    `json:"kostenstelle_id"`
	// PruefintervallMonate ist der Abstand der wiederkehrenden Pruefung.
	// nil = keine wiederkehrende Pruefung.
	PruefintervallMonate *int       `json:"pruefintervall_monate"`
	Notiz                string     `json:"notiz"`
	AusgemustertAm       *time.Time `json:"ausgemustert_am"`
	AusgemustertGrund    string     `json:"ausgemustert_grund"`
	CreatedAt            time.Time  `json:"created_at"`
}

// AssetInput sind die Stammdaten eines Assets - beim Anlegen und beim
// Aendern. Aendern ist ein VOLLSTAENDIGER Ersatz: bei optionalen Angaben
// (Preis, Kostenstelle, Pruefintervall) waere "nicht genannt" sonst von
// "entfernen" nicht zu unterscheiden (wie QualificationTypeUpdate).
// Die Ausmusterung gehoert nicht dazu - sie hat einen eigenen Aufruf.
type AssetInput struct {
	Inventarnummer       string     `json:"inventarnummer"`
	Bezeichnung          string     `json:"bezeichnung"`
	Kategorie            string     `json:"kategorie"`
	Seriennummer         string     `json:"seriennummer"`
	Hersteller           string     `json:"hersteller"`
	Modell               string     `json:"modell"`
	Standort             string     `json:"standort"`
	Anschaffungsdatum    *time.Time `json:"anschaffungsdatum"`
	Anschaffungspreis    *float64   `json:"anschaffungspreis"`
	KostenstelleID       *string    `json:"kostenstelle_id"`
	PruefintervallMonate *int       `json:"pruefintervall_monate"`
	Notiz                string     `json:"notiz"`
}

// ListFilter schraenkt die Liste ein. Leere Kategorie = alle.
type ListFilter struct {
	Kategorie           string
	IncludeAusgemustert bool
}

type Service struct{ pg *pgxpool.Pool }

func NewService(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

const assetColumns = `id, inventarnummer, bezeichnung, kategorie, seriennummer, hersteller, modell, standort,
	anschaffungsdatum, anschaffungspreis::float8, kostenstelle_id, pruefintervall_monate, notiz,
	ausgemustert_am, ausgemustert_grund, created_at`

func scanAsset(row pgx.Row) (*Asset, error) {
	var a Asset
	if err := row.Scan(&a.ID, &a.Inventarnummer, &a.Bezeichnung, &a.Kategorie, &a.Seriennummer, &a.Hersteller, &a.Modell, &a.Standort,
		&a.Anschaffungsdatum, &a.Anschaffungspreis, &a.KostenstelleID, &a.PruefintervallMonate, &a.Notiz,
		&a.AusgemustertAm, &a.AusgemustertGrund, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// normalizeAssetInput entfernt Rand-Leerzeichen und macht eine leere
// Kostenstelle zu "keine". Die Inventarnummer wird NICHT in
// Grossbuchstaben umgewandelt: sie ist die Beschriftung am Stueck und wird
// so gespeichert, wie der Betrieb sie vergibt.
func normalizeAssetInput(in AssetInput) AssetInput {
	in.Inventarnummer = strings.TrimSpace(in.Inventarnummer)
	in.Bezeichnung = strings.TrimSpace(in.Bezeichnung)
	in.Kategorie = strings.TrimSpace(in.Kategorie)
	in.Seriennummer = strings.TrimSpace(in.Seriennummer)
	in.Hersteller = strings.TrimSpace(in.Hersteller)
	in.Modell = strings.TrimSpace(in.Modell)
	in.Standort = strings.TrimSpace(in.Standort)
	if in.KostenstelleID != nil {
		k := strings.TrimSpace(*in.KostenstelleID)
		if k == "" {
			in.KostenstelleID = nil
		} else {
			in.KostenstelleID = &k
		}
	}
	if in.Anschaffungsdatum != nil {
		d := dateOnly(*in.Anschaffungsdatum)
		in.Anschaffungsdatum = &d
	}
	return in
}

// ValidateAssetInput prueft die Stammdaten ohne Datenbank. Erwartet eine
// bereits normalisierte Eingabe.
func ValidateAssetInput(in AssetInput) error {
	if in.Inventarnummer == "" {
		return errors.New("Inventarnummer erforderlich")
	}
	if len([]rune(in.Inventarnummer)) > maxInventarnummerLen {
		return errors.New("Inventarnummer darf höchstens 64 Zeichen haben")
	}
	if in.Bezeichnung == "" {
		return errors.New("Bezeichnung erforderlich")
	}
	if !kategorien[in.Kategorie] {
		return errors.New("Ungültige Kategorie")
	}
	if p := in.Anschaffungspreis; p != nil && (math.IsNaN(*p) || math.IsInf(*p, 0) || *p < 0) {
		return errors.New("Anschaffungspreis darf nicht negativ sein")
	}
	if in.PruefintervallMonate != nil && *in.PruefintervallMonate <= 0 {
		return errors.New("Prüfintervall muss größer als 0 sein")
	}
	return nil
}

// checkKostenstelle stellt sicher, dass eine angegebene Kostenstelle zum
// Mandanten gehoert - die Datenbank kann das nicht (cost_centers.id ist
// global eindeutig). Wie projects.SetKostenstelle.
func checkKostenstelle(ctx context.Context, q pgx.Tx, kostenstelleID *string, companyID string) error {
	if kostenstelleID == nil {
		return nil
	}
	var owned bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cost_centers WHERE id=$1 AND company_id=$2)`, *kostenstelleID, companyID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return errors.New("Kostenstelle nicht gefunden")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, in AssetInput, companyID, actorUserID string) (*Asset, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in = normalizeAssetInput(in)
	if err := ValidateAssetInput(in); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := checkKostenstelle(ctx, tx, in.KostenstelleID, companyID); err != nil {
		return nil, err
	}
	a, err := scanAsset(tx.QueryRow(ctx, `
		INSERT INTO assets (id, company_id, inventarnummer, bezeichnung, kategorie, seriennummer, hersteller, modell, standort,
			anschaffungsdatum, anschaffungspreis, kostenstelle_id, pruefintervall_monate, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,''))
		RETURNING `+assetColumns,
		uuid.New(), companyID, in.Inventarnummer, in.Bezeichnung, in.Kategorie, in.Seriennummer, in.Hersteller, in.Modell, in.Standort,
		in.Anschaffungsdatum, in.Anschaffungspreis, in.KostenstelleID, in.PruefintervallMonate, in.Notiz, actorUserID))
	if err != nil {
		if isUniqueViolation(err, inventarnummerConstraint) {
			return nil, errors.New("Asset mit dieser Inventarnummer existiert bereits")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, companyID string) (*Asset, error) {
	a, err := scanAsset(s.pg.QueryRow(ctx, `SELECT `+assetColumns+` FROM assets WHERE id=$1 AND company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Asset nicht gefunden")
		}
		return nil, err
	}
	return a, nil
}

// List liefert den Bestand des Mandanten nach Inventarnummer sortiert.
// Ausgemusterte nur auf Wunsch.
func (s *Service) List(ctx context.Context, companyID string, f ListFilter) ([]Asset, error) {
	kategorie := strings.TrimSpace(f.Kategorie)
	if kategorie != "" && !kategorien[kategorie] {
		return nil, errors.New("Ungültige Kategorie")
	}
	rows, err := s.pg.Query(ctx, `SELECT `+assetColumns+` FROM assets
		WHERE company_id=$1 AND ($2 = '' OR kategorie = $2) AND (ausgemustert_am IS NULL OR $3)
		ORDER BY inventarnummer, id`, companyID, kategorie, f.IncludeAusgemustert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Asset, 0)
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// Update ersetzt die Stammdaten eines Assets. Auch ein ausgemustertes
// Asset darf berichtigt werden (z. B. eine falsch erfasste
// Seriennummer); die Ausmusterung selbst bleibt davon unberuehrt.
//
// Ein geaendertes Pruefintervall wirkt nur auf Pruefungen, die DANACH
// erfasst werden: die naechste Faelligkeit steht an der Pruefung (ADR
// 0028, Migration 089).
func (s *Service) Update(ctx context.Context, id uuid.UUID, in AssetInput, companyID string) (*Asset, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in = normalizeAssetInput(in)
	if err := ValidateAssetInput(in); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := checkKostenstelle(ctx, tx, in.KostenstelleID, companyID); err != nil {
		return nil, err
	}
	a, err := scanAsset(tx.QueryRow(ctx, `
		UPDATE assets
		   SET inventarnummer=$3, bezeichnung=$4, kategorie=$5, seriennummer=$6, hersteller=$7, modell=$8, standort=$9,
		       anschaffungsdatum=$10, anschaffungspreis=$11, kostenstelle_id=$12, pruefintervall_monate=$13, notiz=$14
		 WHERE id=$1 AND company_id=$2
		RETURNING `+assetColumns,
		id, companyID, in.Inventarnummer, in.Bezeichnung, in.Kategorie, in.Seriennummer, in.Hersteller, in.Modell, in.Standort,
		in.Anschaffungsdatum, in.Anschaffungspreis, in.KostenstelleID, in.PruefintervallMonate, in.Notiz))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Asset nicht gefunden")
		}
		if isUniqueViolation(err, inventarnummerConstraint) {
			return nil, errors.New("Asset mit dieser Inventarnummer existiert bereits")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

// ValidateRetirement prueft eine Ausmusterung ohne Datenbank. today wird
// uebergeben, damit die Regel testbar ist; verglichen werden Kalendertage.
func ValidateRetirement(datum time.Time, grund string, anschaffungsdatum *time.Time, today time.Time) error {
	if datum.IsZero() {
		return errors.New("Ausmusterungsdatum erforderlich")
	}
	if strings.TrimSpace(grund) == "" {
		return errors.New("Grund der Ausmusterung erforderlich")
	}
	// Ausgemustert wird, was ausgemustert IST - kein Vormerken.
	if dateOnly(datum).After(dateOnly(today)) {
		return errors.New("Ausmusterungsdatum darf nicht in der Zukunft liegen")
	}
	if anschaffungsdatum != nil && dateOnly(datum).Before(dateOnly(*anschaffungsdatum)) {
		return errors.New("Ausmusterungsdatum darf nicht vor dem Anschaffungsdatum liegen")
	}
	return nil
}

// Retire mustert ein Asset aus. Endgueltig: ein zweiter Aufruf wird
// abgelehnt, eine Rueckgaengig-Funktion gibt es nicht (ein irrtuemlich
// ausgemustertes Stueck wird neu angelegt - die Historie des alten bleibt).
//
// Ein Stueck, das noch ausgegeben ist, wird nicht ausgemustert: erst
// zuruecknehmen. Damit zwischen Pruefung und Ausmusterung keine Ausgabe
// dazwischenkommt, wird die Asset-Zeile gesperrt. DIESELBE Sperre muss
// die Ausgabe (F.3.4) nehmen, bevor sie prueft, ob das Asset ausgemustert
// ist.
func (s *Service) Retire(ctx context.Context, id uuid.UUID, datum time.Time, grund string, companyID string) (*Asset, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	grund = strings.TrimSpace(grund)
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ausgemustertAm, anschaffungsdatum *time.Time
	if err := tx.QueryRow(ctx, `SELECT ausgemustert_am, anschaffungsdatum FROM assets WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).
		Scan(&ausgemustertAm, &anschaffungsdatum); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Asset nicht gefunden")
		}
		return nil, err
	}
	if ausgemustertAm != nil {
		return nil, errors.New("Asset ist bereits ausgemustert")
	}
	if err := ValidateRetirement(datum, grund, anschaffungsdatum, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	var offen bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM asset_assignments WHERE asset_id=$1 AND zurueckgegeben_am IS NULL AND status='active')`, id).Scan(&offen); err != nil {
		return nil, err
	}
	if offen {
		return nil, errors.New("Asset ist noch ausgegeben - erst zurücknehmen")
	}
	a, err := scanAsset(tx.QueryRow(ctx, `
		UPDATE assets SET ausgemustert_am=$3, ausgemustert_grund=$4
		 WHERE id=$1 AND company_id=$2
		RETURNING `+assetColumns, id, companyID, dateOnly(datum), grund))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a, nil
}
