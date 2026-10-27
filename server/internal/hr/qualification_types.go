package hr

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Katalog der Qualifikationen, Unterweisungen und Beauftragungen (ADR 0027,
// Backlog F.2.3). Der Katalog gehoert dem Mandanten: er legt Arten an,
// aendert sie und legt sie still. Geloescht wird nichts - Nachweise
// verweisen auf die Arten.

const (
	QualificationKategorieQualifikation = "qualifikation"
	QualificationKategorieUnterweisung  = "unterweisung"
	QualificationKategorieBeauftragung  = "beauftragung"
)

var qualificationKategorien = map[string]bool{
	QualificationKategorieQualifikation: true,
	QualificationKategorieUnterweisung:  true,
	QualificationKategorieBeauftragung:  true,
}

// Der Code ist der stabile Schluessel einer Art (Abgleich mit dem
// Vorschlagskatalog, spaeter Bezug aus anderen Modulen). Deshalb ein enger
// Zeichensatz und keine Aenderung nach der Anlage.
var qualificationCodePattern = regexp.MustCompile(`^[A-Z0-9_]{1,40}$`)

// Name des UNIQUE-Constraints aus Migration 088, dessen Verletzung in eine
// fachliche Meldung uebersetzt wird (siehe isUniqueViolation).
const qualificationTypeCodeConstraint = "hr_qualification_types_company_code_key"

type QualificationType struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Kategorie string    `json:"kategorie"`
	// GueltigkeitMonate ist die Frist, nach der ein Nachweis dieser Art
	// erneuert werden muss. nil = unbefristet.
	GueltigkeitMonate *int `json:"gueltigkeit_monate"`
	// BestaetigungMonate: Zwischenbestaetigung alle N Monate (F.10.3,
	// z. B. Schweisserpruefung). nil = keine.
	BestaetigungMonate *int      `json:"bestaetigung_monate"`
	PflichtFuerAlle    bool      `json:"pflicht_fuer_alle"`
	Aktiv              bool      `json:"aktiv"`
	Hinweis            string    `json:"hinweis"`
	CreatedAt          time.Time `json:"created_at"`
}

// QualificationTypeInput sind die Angaben fuer eine neue Art.
type QualificationTypeInput struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	Kategorie          string `json:"kategorie"`
	GueltigkeitMonate  *int   `json:"gueltigkeit_monate"`
	BestaetigungMonate *int   `json:"bestaetigung_monate"`
	PflichtFuerAlle    bool   `json:"pflicht_fuer_alle"`
	Hinweis            string `json:"hinweis"`
}

// QualificationTypeUpdate ersetzt ALLE aenderbaren Angaben einer Art. Ein
// vollstaendiger Ersatz statt eines Patches, weil die Frist auch "keine"
// sein kann: bei einem Patch waere "Frist nicht genannt" von "Frist
// entfernen" nicht zu unterscheiden. Der Code fehlt hier mit Absicht.
type QualificationTypeUpdate struct {
	Name               string `json:"name"`
	Kategorie          string `json:"kategorie"`
	GueltigkeitMonate  *int   `json:"gueltigkeit_monate"`
	BestaetigungMonate *int   `json:"bestaetigung_monate"`
	PflichtFuerAlle    bool   `json:"pflicht_fuer_alle"`
	Aktiv              bool   `json:"aktiv"`
	Hinweis            string `json:"hinweis"`
}

type QualificationTypeService struct{ pg *pgxpool.Pool }

func NewQualificationTypeService(pg *pgxpool.Pool) *QualificationTypeService {
	return &QualificationTypeService{pg: pg}
}

const qualificationTypeColumns = `id, code, name, kategorie, gueltigkeit_monate, bestaetigung_monate, pflicht_fuer_alle, aktiv, hinweis, created_at`

func scanQualificationType(row pgx.Row) (*QualificationType, error) {
	var qt QualificationType
	if err := row.Scan(&qt.ID, &qt.Code, &qt.Name, &qt.Kategorie, &qt.GueltigkeitMonate, &qt.BestaetigungMonate, &qt.PflichtFuerAlle, &qt.Aktiv, &qt.Hinweis, &qt.CreatedAt); err != nil {
		return nil, err
	}
	return &qt, nil
}

// normalizeQualificationCode macht aus einer Eingabe den gespeicherten
// Code: ohne Rand-Leerzeichen, in Grossbuchstaben.
func normalizeQualificationCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func validateQualificationFields(name, kategorie string, gueltigkeitMonate, bestaetigungMonate *int) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("Name erforderlich")
	}
	if !qualificationKategorien[kategorie] {
		return errors.New("Ungültige Kategorie")
	}
	if gueltigkeitMonate != nil && *gueltigkeitMonate <= 0 {
		return errors.New("Gültigkeit muss größer als 0 sein")
	}
	if bestaetigungMonate != nil {
		if *bestaetigungMonate <= 0 {
			return errors.New("Bestätigungsintervall muss größer als 0 sein")
		}
		// Eine Bestaetigung, die erst nach dem Ablauf faellig wird, liefe
		// ins Leere.
		if gueltigkeitMonate != nil && *bestaetigungMonate >= *gueltigkeitMonate {
			return errors.New("Bestätigungsintervall muss kürzer als die Gültigkeit sein")
		}
	}
	return nil
}

func validateQualificationTypeInput(in QualificationTypeInput) error {
	if in.Code == "" {
		return errors.New("Code erforderlich")
	}
	if !qualificationCodePattern.MatchString(in.Code) {
		return errors.New("Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)")
	}
	return validateQualificationFields(in.Name, in.Kategorie, in.GueltigkeitMonate, in.BestaetigungMonate)
}

func (s *QualificationTypeService) Create(ctx context.Context, in QualificationTypeInput, companyID string) (*QualificationType, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in.Code = normalizeQualificationCode(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if err := validateQualificationTypeInput(in); err != nil {
		return nil, err
	}
	qt, err := scanQualificationType(s.pg.QueryRow(ctx, `
		INSERT INTO hr_qualification_types (id, company_id, code, name, kategorie, gueltigkeit_monate, bestaetigung_monate, pflicht_fuer_alle, hinweis)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+qualificationTypeColumns,
		uuid.New(), companyID, in.Code, in.Name, in.Kategorie, in.GueltigkeitMonate, in.BestaetigungMonate, in.PflichtFuerAlle, in.Hinweis))
	if err != nil {
		if isUniqueViolation(err, qualificationTypeCodeConstraint) {
			return nil, errors.New("Qualifikationsart mit diesem Code existiert bereits")
		}
		return nil, err
	}
	return qt, nil
}

func (s *QualificationTypeService) Get(ctx context.Context, id uuid.UUID, companyID string) (*QualificationType, error) {
	qt, err := scanQualificationType(s.pg.QueryRow(ctx, `SELECT `+qualificationTypeColumns+` FROM hr_qualification_types WHERE id=$1 AND company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Qualifikationsart nicht gefunden")
		}
		return nil, err
	}
	return qt, nil
}

// List liefert den Katalog des Mandanten, nach Name sortiert. Ohne
// includeInactive nur die aktiven Arten.
func (s *QualificationTypeService) List(ctx context.Context, companyID string, includeInactive bool) ([]QualificationType, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+qualificationTypeColumns+` FROM hr_qualification_types
		WHERE company_id=$1 AND (aktiv OR $2) ORDER BY name, code`, companyID, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]QualificationType, 0)
	for rows.Next() {
		qt, err := scanQualificationType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *qt)
	}
	return out, rows.Err()
}

// Update ersetzt die aenderbaren Angaben einer Art.
//
// Eine geaenderte Frist wirkt nur auf Nachweise, die DANACH erfasst
// werden: gueltig_bis steht am Nachweis und wird bei seiner Erfassung
// festgelegt (ADR 0027). Bereits ausgestellte Nachweise gelten nicht
// rueckwirkend laenger oder kuerzer.
//
// Stillgelegt wird ueber Aktiv=false. Bestehende Nachweise und
// Anforderungen bleiben davon unberuehrt.
func (s *QualificationTypeService) Update(ctx context.Context, id uuid.UUID, in QualificationTypeUpdate, companyID string) (*QualificationType, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in.Name = strings.TrimSpace(in.Name)
	if err := validateQualificationFields(in.Name, in.Kategorie, in.GueltigkeitMonate, in.BestaetigungMonate); err != nil {
		return nil, err
	}
	qt, err := scanQualificationType(s.pg.QueryRow(ctx, `
		UPDATE hr_qualification_types
		   SET name=$3, kategorie=$4, gueltigkeit_monate=$5, bestaetigung_monate=$6, pflicht_fuer_alle=$7, aktiv=$8, hinweis=$9
		 WHERE id=$1 AND company_id=$2
		RETURNING `+qualificationTypeColumns,
		id, companyID, in.Name, in.Kategorie, in.GueltigkeitMonate, in.BestaetigungMonate, in.PflichtFuerAlle, in.Aktiv, in.Hinweis))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Qualifikationsart nicht gefunden")
		}
		return nil, err
	}
	return qt, nil
}

// AdoptDefaults uebernimmt den Vorschlagskatalog in den Katalog des
// Mandanten und liefert die Arten, die dabei NEU angelegt wurden.
//
// Angelegt wird nur, was fehlt (Abgleich ueber den Code). Eine Art, die
// der Mandant bereits hat - uebernommen und veraendert oder selbst mit
// demselben Code angelegt -, wird nicht angefasst: was im Katalog steht,
// gehoert dem Mandanten. Ein zweiter Aufruf legt deshalb nichts an.
func (s *QualificationTypeService) AdoptDefaults(ctx context.Context, companyID string) ([]QualificationType, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	created := make([]QualificationType, 0)
	for _, d := range DefaultQualificationTypes() {
		// ON CONFLICT DO NOTHING statt einer Vorabfrage: auch zwei
		// gleichzeitige Uebernahmen legen jede Art nur einmal an.
		qt, err := scanQualificationType(tx.QueryRow(ctx, `
			INSERT INTO hr_qualification_types (id, company_id, code, name, kategorie, gueltigkeit_monate, bestaetigung_monate, pflicht_fuer_alle, hinweis)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (company_id, code) DO NOTHING
			RETURNING `+qualificationTypeColumns,
			uuid.New(), companyID, d.Code, d.Name, d.Kategorie, d.GueltigkeitMonate, d.BestaetigungMonate, d.PflichtFuerAlle, d.Hinweis))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue // gab es schon
			}
			return nil, err
		}
		created = append(created, *qt)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return created, nil
}
