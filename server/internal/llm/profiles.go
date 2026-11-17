package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"nalaerp3/internal/secretbox"
)

// Modellprofile je Mandant (ADR 0046 Fragen 1 und 2, Backlog I.8.1a2): ein
// Mandant hinterlegt mehrere Sprachmodelle, der Kalkulator waehlt beim Lauf
// eines. Hauptschalter und Tageslimit gelten fuer alle Profile (Mandant).
// Das erste Profil wird Standardprofil; der Standard wechselt nur, indem ein
// anderes Profil Standard wird.

const (
	maxProfilName   = 100
	DefaultSchwelle = 0.9
)

// Profile ist ein Modellprofil, wie die API es zeigt - ohne Schluessel.
type Profile struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Aktiv           bool       `json:"aktiv"`
	Standard        bool       `json:"standard"`
	Anbieter        string     `json:"anbieter"`
	Endpunkt        string     `json:"endpunkt"`
	Modell          string     `json:"modell"`
	TimeoutS        int        `json:"timeout_s"`
	Schwelle        float64    `json:"schwelle"`
	APIKeyGesetzt   bool       `json:"api_key_gesetzt"`
	APIKeyLesbar    bool       `json:"api_key_lesbar"`
	UpdatedByUserID *string    `json:"updated_by_user_id"`
	UpdatedAt       *time.Time `json:"updated_at"`
}

// ProfileInput ersetzt ein Profil vollstaendig (ohne Standard, siehe
// SetStandardProfile). APIKey "" laesst den gespeicherten stehen;
// APIKeyLoeschen entfernt ihn. Schwelle nil = 0,9.
type ProfileInput struct {
	Name           string   `json:"name"`
	Aktiv          bool     `json:"aktiv"`
	Anbieter       string   `json:"anbieter"`
	Endpunkt       string   `json:"endpunkt"`
	Modell         string   `json:"modell"`
	APIKey         string   `json:"api_key"`
	APIKeyLoeschen bool     `json:"api_key_loeschen"`
	TimeoutS       int      `json:"timeout_s"`
	Schwelle       *float64 `json:"schwelle"`
}

// settings: die modellbezogenen Felder pruefen Normalize- und
// ValidateSettingsInput (das Tageslimit gehoert nicht zum Profil).
func (in ProfileInput) settings() SettingsInput {
	return SettingsInput{Aktiv: in.Aktiv, Anbieter: in.Anbieter, Endpunkt: in.Endpunkt, Modell: in.Modell, APIKey: in.APIKey,
		APIKeyLoeschen: in.APIKeyLoeschen, TimeoutS: in.TimeoutS, Tageslimit: DefaultTageslimit}
}

// NormalizeProfileInput wie NormalizeSettingsInput, dazu Name und Schwelle.
func NormalizeProfileInput(in ProfileInput) ProfileInput {
	n := NormalizeSettingsInput(in.settings())
	in.Anbieter, in.Endpunkt, in.Modell, in.TimeoutS = n.Anbieter, n.Endpunkt, n.Modell, n.TimeoutS
	in.Name = strings.TrimSpace(in.Name)
	if in.Schwelle == nil {
		d := DefaultSchwelle
		in.Schwelle = &d
	}
	return in
}

// ValidateProfileInput prueft eine normalisierte Eingabe ohne Datenbank.
func ValidateProfileInput(in ProfileInput) error {
	if in.Name == "" || utf8.RuneCountInString(in.Name) > maxProfilName || strings.IndexFunc(in.Name, unicode.IsControl) >= 0 {
		return fmt.Errorf("Name des Profils ungültig (1 bis %d Zeichen)", maxProfilName)
	}
	if in.Schwelle == nil || !(*in.Schwelle >= 0 && *in.Schwelle <= 1) {
		return errors.New("Schwelle ungültig (0 bis 1)")
	}
	return ValidateSettingsInput(in.settings())
}

const profileColumns = `id, name, aktiv, standard, anbieter, endpunkt, modell, api_key_enc, timeout_s, schwelle::float8,
	updated_by_user_id, updated_at`

func scanProfile(row pgx.Row) (*Profile, []byte, error) {
	var p Profile
	var enc []byte
	err := row.Scan(&p.ID, &p.Name, &p.Aktiv, &p.Standard, &p.Anbieter, &p.Endpunkt, &p.Modell, &enc, &p.TimeoutS, &p.Schwelle,
		&p.UpdatedByUserID, &p.UpdatedAt)
	return &p, enc, err
}

func (s *SettingsService) profileView(p *Profile, enc []byte, companyID string) *Profile {
	p.APIKeyGesetzt = enc != nil
	if enc != nil {
		_, err := s.box.Open(enc, apiKeyAAD(companyID))
		p.APIKeyLesbar = err == nil
	}
	return p
}

// ErrProfileNotFound: kein Profil mit dieser ID beim Mandanten.
var ErrProfileNotFound = errors.New("Profil nicht gefunden")

func isProfileNameTaken(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "company_llm_profiles_name_unique"
}

func checkCompany(companyID string) error {
	if strings.TrimSpace(companyID) == "" {
		return errors.New("Mandant erforderlich")
	}
	return nil
}

func actorOf(actorUserID string) *string {
	if strings.TrimSpace(actorUserID) == "" {
		return nil
	}
	return &actorUserID
}

// ListProfiles liefert die Profile des Mandanten, Standardprofil zuerst,
// sonst nach Name.
func (s *SettingsService) ListProfiles(ctx context.Context, companyID string) ([]Profile, error) {
	if err := checkCompany(companyID); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `SELECT `+profileColumns+` FROM company_llm_profiles WHERE company_id=$1
		ORDER BY standard DESC, lower(name), id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		p, enc, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s.profileView(p, enc, companyID))
	}
	return out, rows.Err()
}

// GetProfile liefert ein Profil des Mandanten.
func (s *SettingsService) GetProfile(ctx context.Context, companyID, id string) (*Profile, error) {
	if err := checkCompany(companyID); err != nil {
		return nil, err
	}
	p, enc, err := s.loadProfile(ctx, s.pg.QueryRow, companyID, id, "")
	if err != nil {
		return nil, err
	}
	return s.profileView(p, enc, companyID), nil
}

type queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row

// loadProfile: id "" = Standardprofil; suffix z. B. " FOR UPDATE".
func (s *SettingsService) loadProfile(ctx context.Context, q queryRowFunc, companyID, id, suffix string) (*Profile, []byte, error) {
	var row pgx.Row
	if id == "" {
		row = q(ctx, `SELECT `+profileColumns+` FROM company_llm_profiles WHERE company_id=$1 AND standard`+suffix, companyID)
	} else {
		if _, err := uuid.Parse(id); err != nil {
			return nil, nil, ErrProfileNotFound
		}
		row = q(ctx, `SELECT `+profileColumns+` FROM company_llm_profiles WHERE company_id=$1 AND id=$2`+suffix, companyID, id)
	}
	p, enc, err := scanProfile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrProfileNotFound
	}
	return p, enc, err
}

// sealKey liefert den zu speichernden Schluessel: neu, geloescht oder alt.
func (s *SettingsService) sealKey(in ProfileInput, old []byte, companyID string) ([]byte, error) {
	switch {
	case in.APIKey != "":
		enc, err := s.box.Seal([]byte(in.APIKey), apiKeyAAD(companyID))
		if errors.Is(err, secretbox.ErrNoKey) {
			return nil, errors.New("API-Schlüssel kann nicht gespeichert werden: SETTINGS_ENCRYPTION_KEY fehlt")
		}
		return enc, err
	case in.APIKeyLoeschen:
		return nil, nil
	}
	return old, nil
}

// CreateProfile legt ein Profil an; das erste des Mandanten wird Standard.
func (s *SettingsService) CreateProfile(ctx context.Context, in ProfileInput, actorUserID, companyID string) (*Profile, error) {
	return s.saveProfile(ctx, "", in, actorUserID, companyID)
}

// UpdateProfile ersetzt ein Profil. Das letzte eingeschaltete Profil kann
// nicht ausgeschaltet werden, solange der Hauptschalter an ist.
func (s *SettingsService) UpdateProfile(ctx context.Context, id string, in ProfileInput, actorUserID, companyID string) (*Profile, error) {
	if id == "" {
		return nil, ErrProfileNotFound
	}
	return s.saveProfile(ctx, id, in, actorUserID, companyID)
}

func (s *SettingsService) saveProfile(ctx context.Context, id string, in ProfileInput, actorUserID, companyID string) (*Profile, error) {
	if err := checkCompany(companyID); err != nil {
		return nil, err
	}
	in = NormalizeProfileInput(in)
	if err := ValidateProfileInput(in); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Sperre am Mandanten: erstes Profil und Hauptschalter ohne Wettlauf.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM company_profiles WHERE id=$1 FOR UPDATE`, companyID); err != nil {
		return nil, err
	}
	var old []byte
	if id != "" {
		cur, enc, err := s.loadProfile(ctx, tx.QueryRow, companyID, id, " FOR UPDATE")
		if err != nil {
			return nil, err
		}
		old = enc
		if cur.Aktiv && !in.Aktiv {
			if err := lastActiveCheck(ctx, tx, companyID, id); err != nil {
				return nil, err
			}
		}
	}
	enc, err := s.sealKey(in, old, companyID)
	if err != nil {
		return nil, err
	}
	if in.Aktiv && in.Anbieter != ProviderOpenAICompatible && enc == nil {
		return nil, errors.New("Für das Sprachmodell fehlt: API-Schlüssel")
	}
	actor := actorOf(actorUserID)
	if id == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO company_llm_profiles (company_id, name, aktiv, standard, anbieter, endpunkt, modell, api_key_enc, timeout_s,
				schwelle, updated_by_user_id, updated_at)
			VALUES ($1,$2,$3,NOT EXISTS (SELECT 1 FROM company_llm_profiles WHERE company_id=$1),$4,$5,$6,$7,$8,$9,$10,now())
			RETURNING id`,
			companyID, in.Name, in.Aktiv, in.Anbieter, in.Endpunkt, in.Modell, enc, in.TimeoutS, *in.Schwelle, actor).Scan(&id)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE company_llm_profiles SET name=$3, aktiv=$4, anbieter=$5, endpunkt=$6, modell=$7, api_key_enc=$8, timeout_s=$9,
				schwelle=$10, updated_by_user_id=$11, updated_at=now()
			WHERE company_id=$1 AND id=$2`,
			companyID, id, in.Name, in.Aktiv, in.Anbieter, in.Endpunkt, in.Modell, enc, in.TimeoutS, *in.Schwelle, actor)
	}
	if isProfileNameTaken(err) {
		return nil, errors.New("Name des Profils bereits vergeben")
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, companyID, id)
}

// lastActiveCheck: bei eingeschaltetem Hauptschalter muss ausser exceptID
// ein weiteres Profil eingeschaltet sein.
func lastActiveCheck(ctx context.Context, tx pgx.Tx, companyID, exceptID string) error {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT aktiv FROM company_llm_settings WHERE company_id=$1), false)
		AND NOT EXISTS (SELECT 1 FROM company_llm_profiles WHERE company_id=$1 AND aktiv AND id<>$2)`, companyID, exceptID).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return errors.New("Letztes eingeschaltetes Profil: zuerst das Sprachmodell ausschalten")
	}
	return nil
}

// SetStandardProfile macht ein Profil zum Standardprofil (das bisherige
// verliert den Standard).
func (s *SettingsService) SetStandardProfile(ctx context.Context, id, actorUserID, companyID string) (*Profile, error) {
	if err := checkCompany(companyID); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM company_profiles WHERE id=$1 FOR UPDATE`, companyID); err != nil {
		return nil, err
	}
	cur, _, err := s.loadProfile(ctx, tx.QueryRow, companyID, id, " FOR UPDATE")
	if err != nil {
		return nil, err
	}
	if !cur.Standard {
		actor := actorOf(actorUserID)
		// zwei Anweisungen: der eindeutige Index prueft je Zeile sofort
		if _, err := tx.Exec(ctx, `UPDATE company_llm_profiles SET standard=false, updated_by_user_id=$2, updated_at=now()
			WHERE company_id=$1 AND standard`, companyID, actor); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE company_llm_profiles SET standard=true, updated_by_user_id=$3, updated_at=now()
			WHERE company_id=$1 AND id=$2`, companyID, id, actor); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, companyID, id)
}

// Mandant: Hauptschalter und Tageslimit, gueltig fuer alle Profile.
type Mandant struct {
	Aktiv        bool       `json:"aktiv"`
	Tageslimit   int        `json:"tageslimit"`
	AufrufeHeute int        `json:"aufrufe_heute"`
	UpdatedAt    *time.Time `json:"updated_at"`
	// SchluesselEingerichtet: SETTINGS_ENCRYPTION_KEY ist gesetzt.
	SchluesselEingerichtet bool    `json:"schluessel_eingerichtet"`
	UpdatedByUserID        *string `json:"updated_by_user_id"`
}

// MandantInput: Tageslimit 0 = 500.
type MandantInput struct {
	Aktiv      bool `json:"aktiv"`
	Tageslimit int  `json:"tageslimit"`
}

// GetMandant liefert Hauptschalter und Tageslimit; ohne gespeicherte die
// Standardwerte (aus, 500).
func (s *SettingsService) GetMandant(ctx context.Context, companyID string) (*Mandant, error) {
	if err := checkCompany(companyID); err != nil {
		return nil, err
	}
	m := Mandant{Tageslimit: DefaultTageslimit, SchluesselEingerichtet: s.box != nil}
	err := s.pg.QueryRow(ctx, `SELECT s.aktiv, s.tageslimit, s.updated_by_user_id, s.updated_at FROM company_llm_settings s
		WHERE s.company_id=$1`, companyID).Scan(&m.Aktiv, &m.Tageslimit, &m.UpdatedByUserID, &m.UpdatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err := s.pg.QueryRow(ctx, `SELECT COALESCE((SELECT aufrufe FROM company_llm_usage WHERE company_id=$1 AND tag=current_date), 0)`,
		companyID).Scan(&m.AufrufeHeute); err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveMandant speichert Hauptschalter und Tageslimit. Einschalten verlangt
// mindestens ein eingeschaltetes Profil.
func (s *SettingsService) SaveMandant(ctx context.Context, in MandantInput, actorUserID, companyID string) (*Mandant, error) {
	if err := checkCompany(companyID); err != nil {
		return nil, err
	}
	if in.Tageslimit == 0 {
		in.Tageslimit = DefaultTageslimit
	}
	if in.Tageslimit < 1 || in.Tageslimit > maxTageslimit {
		return nil, fmt.Errorf("Tageslimit ungültig (1 bis %d Aufrufe)", maxTageslimit)
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM company_profiles WHERE id=$1 FOR UPDATE`, companyID); err != nil {
		return nil, err
	}
	if in.Aktiv {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM company_llm_profiles WHERE company_id=$1 AND aktiv)`, companyID).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("Zum Einschalten fehlt ein eingeschaltetes Profil")
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO company_llm_settings (company_id, aktiv, tageslimit, updated_by_user_id, updated_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (company_id) DO UPDATE SET
			aktiv=EXCLUDED.aktiv, tageslimit=EXCLUDED.tageslimit,
			updated_by_user_id=EXCLUDED.updated_by_user_id, updated_at=now()`,
		companyID, in.Aktiv, in.Tageslimit, actorOf(actorUserID)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetMandant(ctx, companyID)
}

// ProfilAbschrift haelt fest, mit welchem Profil ein Lauf arbeitet (ADR 0046
// Frage 2): bleibt am Lauf, auch wenn das Profil spaeter geaendert wird.
type ProfilAbschrift struct {
	ProfilID string  `json:"profil_id"`
	Name     string  `json:"name"`
	Anbieter string  `json:"anbieter"`
	Modell   string  `json:"modell"`
	Schwelle float64 `json:"schwelle"`
}

// ProfileClient liefert den Client fuer ein Profil (id "" = Standardprofil)
// und die Abschrift fuer den Lauf: nur bei eingeschaltetem Hauptschalter und
// eingeschaltetem Profil.
func (s *SettingsService) ProfileClient(ctx context.Context, companyID, id string, recorder Recorder) (*Client, *ProfilAbschrift, error) {
	return s.profileClientFor(ctx, companyID, id, recorder, true)
}

// TestProfileClient wie ProfileClient, aber auch ausgeschaltet - fuer den
// Verbindungstest. Auch Testaufrufe zaehlen zum Tageslimit.
func (s *SettingsService) TestProfileClient(ctx context.Context, companyID, id string, recorder Recorder) (*Client, *ProfilAbschrift, error) {
	return s.profileClientFor(ctx, companyID, id, recorder, false)
}

func (s *SettingsService) profileClientFor(ctx context.Context, companyID, id string, recorder Recorder, requireActive bool) (*Client, *ProfilAbschrift, error) {
	m, err := s.GetMandant(ctx, companyID)
	if err != nil {
		return nil, nil, err
	}
	p, enc, err := s.loadProfile(ctx, s.pg.QueryRow, companyID, id, "")
	if err != nil {
		return nil, nil, err
	}
	if requireActive && (!m.Aktiv || !p.Aktiv) {
		return nil, nil, errors.New("Sprachmodell ist nicht eingeschaltet")
	}
	set := Settings{Anbieter: p.Anbieter, Endpunkt: p.Endpunkt, Modell: p.Modell, TimeoutS: p.TimeoutS}
	c, err := s.budgetClient(companyID, set, enc, m.Tageslimit, recorder)
	if err != nil {
		return nil, nil, err
	}
	return c, &ProfilAbschrift{ProfilID: p.ID, Name: p.Name, Anbieter: p.Anbieter, Modell: p.Modell, Schwelle: p.Schwelle}, nil
}
