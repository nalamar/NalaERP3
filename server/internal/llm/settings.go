package llm

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/secretbox"
)

// Einstellungen je Mandant (ADR 0043 Fragen 3 und 6, Backlog I.3.5a1),
// Muster wie der Mail-Zugang (ADR 0033): der API-Schluessel steht nur
// verschluesselt in company_llm_settings und verlaesst den Dienst nur im
// Provider - nie ueber JSON, nie im Log.

const (
	ProviderOpenAICompatible = "openai_compatible"
	ProviderAnthropic        = "anthropic"
	ProviderMistral          = "mistral"

	DefaultTageslimit = 500
	maxTimeoutSeconds = 1800
	maxTageslimit     = 100000
)

var providers = map[string]bool{ProviderOpenAICompatible: true, ProviderAnthropic: true, ProviderMistral: true}

// Settings ist die Einstellung, wie die API sie zeigt - ohne Schluessel.
type Settings struct {
	Eingerichtet bool   `json:"eingerichtet"`
	Aktiv        bool   `json:"aktiv"`
	Anbieter     string `json:"anbieter"`
	Endpunkt     string `json:"endpunkt"`
	Modell       string `json:"modell"`
	TimeoutS     int    `json:"timeout_s"`
	Tageslimit   int    `json:"tageslimit"`
	// AufrufeHeute: bisherige Aufrufe am heutigen Tag (Tageslimit).
	AufrufeHeute  int  `json:"aufrufe_heute"`
	APIKeyGesetzt bool `json:"api_key_gesetzt"`
	APIKeyLesbar  bool `json:"api_key_lesbar"`
	// SchluesselEingerichtet: SETTINGS_ENCRYPTION_KEY ist gesetzt.
	SchluesselEingerichtet bool       `json:"schluessel_eingerichtet"`
	UpdatedByUserID        *string    `json:"updated_by_user_id"`
	UpdatedAt              *time.Time `json:"updated_at"`
}

// SettingsInput ersetzt die Einstellung vollstaendig. APIKey "" laesst den
// gespeicherten stehen; APIKeyLoeschen entfernt ihn.
type SettingsInput struct {
	Aktiv          bool   `json:"aktiv"`
	Anbieter       string `json:"anbieter"`
	Endpunkt       string `json:"endpunkt"`
	Modell         string `json:"modell"`
	APIKey         string `json:"api_key"`
	APIKeyLoeschen bool   `json:"api_key_loeschen"`
	TimeoutS       int    `json:"timeout_s"`
	Tageslimit     int    `json:"tageslimit"`
}

func apiKeyAAD(companyID string) []byte { return []byte("llm-api-key:" + companyID) }

// NormalizeSettingsInput entfernt Leerraum am Rand (nicht beim Schluessel),
// schreibt den Anbieter klein und setzt Standardwerte.
func NormalizeSettingsInput(in SettingsInput) SettingsInput {
	in.Anbieter = strings.ToLower(strings.TrimSpace(in.Anbieter))
	if in.Anbieter == "" {
		in.Anbieter = ProviderOpenAICompatible
	}
	in.Endpunkt = strings.TrimSpace(in.Endpunkt)
	in.Modell = strings.TrimSpace(in.Modell)
	if in.TimeoutS == 0 {
		in.TimeoutS = int(DefaultTimeout / time.Second)
	}
	if in.Tageslimit == 0 {
		in.Tageslimit = DefaultTageslimit
	}
	return in
}

// ValidateSettingsInput prueft eine normalisierte Eingabe ohne Datenbank.
// Ob ein Schluessel vorliegt, entscheidet Save (gespeicherter zaehlt).
func ValidateSettingsInput(in SettingsInput) error {
	if !providers[in.Anbieter] {
		return errors.New("Anbieter ungültig (openai_compatible, anthropic oder mistral)")
	}
	if in.Endpunkt != "" {
		u, err := url.Parse(in.Endpunkt)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || len(in.Endpunkt) > 500 ||
			strings.ContainsAny(in.Endpunkt, " \t\r\n") {
			return errors.New("Endpunkt ungültig (vollständige http- oder https-Adresse, z. B. http://localhost:11434/v1)")
		}
	}
	if len(in.Modell) > 200 || strings.ContainsAny(in.Modell, " \t\r\n") {
		return errors.New("Modell ungültig")
	}
	if len(in.APIKey) > 1000 || strings.ContainsAny(in.APIKey, "\r\n") {
		return errors.New("API-Schlüssel ungültig")
	}
	if in.APIKey != "" && in.APIKeyLoeschen {
		return errors.New("API-Schlüssel entweder neu setzen oder löschen")
	}
	if in.TimeoutS < 1 || in.TimeoutS > maxTimeoutSeconds {
		return fmt.Errorf("Zeitlimit ungültig (1 bis %d Sekunden)", maxTimeoutSeconds)
	}
	if in.Tageslimit < 1 || in.Tageslimit > maxTageslimit {
		return fmt.Errorf("Tageslimit ungültig (1 bis %d Aufrufe)", maxTageslimit)
	}
	if in.Aktiv && in.Anbieter == ProviderOpenAICompatible && (in.Endpunkt == "" || in.Modell == "") {
		return errors.New("Für das Sprachmodell fehlt: Endpunkt und Modell")
	}
	return nil
}

// SettingsService verwaltet die Einstellungen und baut daraus den Client.
type SettingsService struct {
	pg  *pgxpool.Pool
	box *secretbox.Box
}

// NewSettingsService: box nil = kein Schluessel eingerichtet.
func NewSettingsService(pg *pgxpool.Pool, box *secretbox.Box) *SettingsService {
	return &SettingsService{pg: pg, box: box}
}

type settingsRow struct {
	s   Settings
	enc []byte
}

const settingsSelect = `SELECT aktiv, anbieter, endpunkt, modell, api_key_enc, timeout_s, tageslimit, updated_by_user_id, updated_at,
	COALESCE((SELECT aufrufe FROM company_llm_usage u WHERE u.company_id=$1 AND u.tag=current_date), 0)
	FROM company_llm_settings WHERE company_id=$1`

func loadSettings(q pgx.Row) (*settingsRow, error) {
	var r settingsRow
	err := q.Scan(&r.s.Aktiv, &r.s.Anbieter, &r.s.Endpunkt, &r.s.Modell, &r.enc, &r.s.TimeoutS, &r.s.Tageslimit,
		&r.s.UpdatedByUserID, &r.s.UpdatedAt, &r.s.AufrufeHeute)
	if errors.Is(err, pgx.ErrNoRows) {
		return &settingsRow{s: Settings{Anbieter: ProviderOpenAICompatible, TimeoutS: int(DefaultTimeout / time.Second), Tageslimit: DefaultTageslimit}}, nil
	}
	if err != nil {
		return nil, err
	}
	r.s.Eingerichtet = true
	return &r, nil
}

func (s *SettingsService) view(r *settingsRow, companyID string) *Settings {
	out := r.s
	out.SchluesselEingerichtet = s.box != nil
	out.APIKeyGesetzt = r.enc != nil
	if r.enc != nil {
		_, err := s.box.Open(r.enc, apiKeyAAD(companyID))
		out.APIKeyLesbar = err == nil
	}
	return &out
}

// Get liefert die Einstellung des Mandanten; ohne gespeicherte die
// Standardwerte (Eingerichtet=false).
func (s *SettingsService) Get(ctx context.Context, companyID string) (*Settings, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	r, err := loadSettings(s.pg.QueryRow(ctx, settingsSelect, companyID))
	if err != nil {
		return nil, err
	}
	return s.view(r, companyID), nil
}

// Save ersetzt die Einstellung des Mandanten.
func (s *SettingsService) Save(ctx context.Context, in SettingsInput, actorUserID, companyID string) (*Settings, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in = NormalizeSettingsInput(in)
	if err := ValidateSettingsInput(in); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cur, err := loadSettings(tx.QueryRow(ctx, settingsSelect+` FOR UPDATE`, companyID))
	if err != nil {
		return nil, err
	}
	enc := cur.enc
	switch {
	case in.APIKey != "":
		if enc, err = s.box.Seal([]byte(in.APIKey), apiKeyAAD(companyID)); err != nil {
			if errors.Is(err, secretbox.ErrNoKey) {
				return nil, errors.New("API-Schlüssel kann nicht gespeichert werden: SETTINGS_ENCRYPTION_KEY fehlt")
			}
			return nil, err
		}
	case in.APIKeyLoeschen:
		enc = nil
	}
	if in.Aktiv && in.Anbieter != ProviderOpenAICompatible && enc == nil {
		return nil, errors.New("Für das Sprachmodell fehlt: API-Schlüssel")
	}
	var actor *string
	if strings.TrimSpace(actorUserID) != "" {
		actor = &actorUserID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO company_llm_settings (company_id, aktiv, anbieter, endpunkt, modell, api_key_enc, timeout_s, tageslimit, updated_by_user_id, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
		ON CONFLICT (company_id) DO UPDATE SET
			aktiv=EXCLUDED.aktiv, anbieter=EXCLUDED.anbieter, endpunkt=EXCLUDED.endpunkt, modell=EXCLUDED.modell,
			api_key_enc=EXCLUDED.api_key_enc, timeout_s=EXCLUDED.timeout_s, tageslimit=EXCLUDED.tageslimit,
			updated_by_user_id=EXCLUDED.updated_by_user_id, updated_at=now()`,
		companyID, in.Aktiv, in.Anbieter, in.Endpunkt, in.Modell, enc, in.TimeoutS, in.Tageslimit, actor); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, companyID)
}

// newProvider baut die Anbindung aus der Einstellung.
func newProvider(set Settings, apiKey string) Provider {
	switch set.Anbieter {
	case ProviderAnthropic:
		return &Anthropic{BaseURL: set.Endpunkt, APIKey: apiKey, Model: set.Modell}
	case ProviderMistral:
		p := NewMistral(apiKey, set.Modell)
		if set.Endpunkt != "" {
			p.BaseURL = set.Endpunkt
		}
		return p
	default:
		return &OpenAICompatible{BaseURL: set.Endpunkt, APIKey: apiKey, Model: set.Modell}
	}
}

// Client liefert den Client fuer den Mandanten: nur wenn eingeschaltet,
// mit dem eingestellten Zeitlimit, hoechstens 2 Wiederholungen und dem
// Tageslimit als Pruefung vor jeder Anfrage. recorder nil = LogRecorder.
func (s *SettingsService) Client(ctx context.Context, companyID string, recorder Recorder) (*Client, error) {
	return s.clientFor(ctx, companyID, recorder, true)
}

// TestClient liefert den Client auch, wenn das Sprachmodell noch nicht
// eingeschaltet ist - fuer den Verbindungstest vor dem Einschalten (wie
// der Testversand beim Mail-Zugang). Unvollstaendige Einstellungen melden
// die Anbindungen selbst. Auch Testaufrufe zaehlen zum Tageslimit.
func (s *SettingsService) TestClient(ctx context.Context, companyID string, recorder Recorder) (*Client, error) {
	return s.clientFor(ctx, companyID, recorder, false)
}

// ErrDailyLimit: das Tageslimit des Mandanten ist erreicht.
var ErrDailyLimit = errors.New("Tageslimit des Sprachmodells erreicht")

func (s *SettingsService) clientFor(ctx context.Context, companyID string, recorder Recorder, requireActive bool) (*Client, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	r, err := loadSettings(s.pg.QueryRow(ctx, settingsSelect, companyID))
	if err != nil {
		return nil, err
	}
	if requireActive && !r.s.Aktiv {
		return nil, errors.New("Sprachmodell ist nicht eingeschaltet")
	}
	if !r.s.Eingerichtet {
		return nil, errors.New("Sprachmodell nicht vollständig eingerichtet")
	}
	apiKey := ""
	if r.enc != nil {
		plain, err := s.box.Open(r.enc, apiKeyAAD(companyID))
		if err != nil {
			return nil, errors.New("API-Schlüssel des Sprachmodells ungültig (nicht lesbar) - bitte neu eintragen")
		}
		apiKey = string(plain)
	}
	limit := r.s.Tageslimit
	c := NewClient(newProvider(r.s, apiKey), Limits{Timeout: time.Duration(r.s.TimeoutS) * time.Second, MaxRetries: DefaultMaxRetries}, recorder)
	return c.WithBudget(func(ctx context.Context) error { return s.reserveCall(ctx, companyID, limit) }), nil
}

// reserveCall zaehlt einen Aufruf fuer heute, solange das Tageslimit nicht
// erreicht ist (atomar in der Datenbank).
func (s *SettingsService) reserveCall(ctx context.Context, companyID string, limit int) error {
	var n int
	err := s.pg.QueryRow(ctx, `
		INSERT INTO company_llm_usage (company_id, tag, aufrufe) VALUES ($1, current_date, 1)
		ON CONFLICT (company_id, tag) DO UPDATE SET aufrufe = company_llm_usage.aufrufe + 1
		WHERE company_llm_usage.aufrufe < $2
		RETURNING aufrufe`, companyID, limit).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w (%d Aufrufe)", ErrDailyLimit, limit)
	}
	return err
}
