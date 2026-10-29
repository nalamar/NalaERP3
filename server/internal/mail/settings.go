// Package mail: Mail-Zugang je Mandant und Versand (ADR 0033, Backlog F.14).
//
// Das Passwort steht nur verschluesselt in company_mail_settings
// (secretbox, AAD "mail-password:<Mandant>") und verlaesst den Dienst nur
// in SMTPConfig fuer den Versand - nie ueber JSON, nie im Log.
package mail

import (
	"context"
	"errors"
	"fmt"
	netmail "net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/secretbox"
)

const (
	SicherheitSSL      = "ssl"
	SicherheitStartTLS = "starttls"
)

// Ueblicher Port je Verschluesselung, wenn keiner angegeben ist.
var defaultPorts = map[string]int{SicherheitSSL: 465, SicherheitStartTLS: 587}

// Settings ist der Mail-Zugang, wie die API ihn zeigt - ohne Passwort.
type Settings struct {
	// Eingerichtet: es gibt eine gespeicherte Einstellung.
	Eingerichtet    bool   `json:"eingerichtet"`
	Aktiv           bool   `json:"aktiv"`
	SMTPHost        string `json:"smtp_host"`
	SMTPPort        int    `json:"smtp_port"`
	Sicherheit      string `json:"sicherheit"`
	Benutzer        string `json:"benutzer"`
	AbsenderAdresse string `json:"absender_adresse"`
	AbsenderName    string `json:"absender_name"`
	PasswortGesetzt bool   `json:"passwort_gesetzt"`
	// PasswortLesbar: das gespeicherte Passwort laesst sich mit dem
	// aktuellen Schluessel entschluesseln. false nach Schluesselwechsel -
	// dann neu eintragen.
	PasswortLesbar bool `json:"passwort_lesbar"`
	// SchluesselEingerichtet: SETTINGS_ENCRYPTION_KEY ist gesetzt; ohne
	// laesst sich kein Passwort speichern.
	SchluesselEingerichtet bool       `json:"schluessel_eingerichtet"`
	UpdatedByUserID        *string    `json:"updated_by_user_id"`
	UpdatedAt              *time.Time `json:"updated_at"`
}

// Input ersetzt die Einstellung vollstaendig. Passwort "" laesst das
// gespeicherte stehen; PasswortLoeschen entfernt es.
type Input struct {
	Aktiv            bool   `json:"aktiv"`
	SMTPHost         string `json:"smtp_host"`
	SMTPPort         int    `json:"smtp_port"`
	Sicherheit       string `json:"sicherheit"`
	Benutzer         string `json:"benutzer"`
	Passwort         string `json:"passwort"`
	PasswortLoeschen bool   `json:"passwort_loeschen"`
	AbsenderAdresse  string `json:"absender_adresse"`
	AbsenderName     string `json:"absender_name"`
}

// SMTPConfig ist der entschluesselte Zugang fuer den Versand. Das Passwort
// wird nie serialisiert und nie ausgegeben.
type SMTPConfig struct {
	Aktiv           bool
	Host            string
	Port            int
	Sicherheit      string
	Benutzer        string
	Passwort        string `json:"-"`
	AbsenderAdresse string
	AbsenderName    string
}

// String verbirgt das Passwort, falls die Struktur doch in ein Log geraet.
func (c SMTPConfig) String() string {
	return fmt.Sprintf("SMTPConfig{%s:%d %s Benutzer=%q Passwort=***}", c.Host, c.Port, c.Sicherheit, c.Benutzer)
}

func (c SMTPConfig) GoString() string { return c.String() }

func passwordAAD(companyID string) []byte { return []byte("mail-password:" + companyID) }

func hasLineBreak(s string) bool { return strings.ContainsAny(s, "\r\n") }

// NormalizeInput entfernt Leerraum am Rand (nicht beim Passwort), schreibt
// Server und Verschluesselung klein und setzt den ueblichen Port.
func NormalizeInput(in Input) Input {
	in.SMTPHost = strings.ToLower(strings.TrimSpace(in.SMTPHost))
	in.Sicherheit = strings.ToLower(strings.TrimSpace(in.Sicherheit))
	if in.Sicherheit == "" {
		in.Sicherheit = SicherheitSSL
	}
	if in.SMTPPort == 0 {
		in.SMTPPort = defaultPorts[in.Sicherheit]
	}
	in.Benutzer = strings.TrimSpace(in.Benutzer)
	in.AbsenderAdresse = strings.TrimSpace(in.AbsenderAdresse)
	in.AbsenderName = strings.TrimSpace(in.AbsenderName)
	return in
}

// ValidateInput prueft eine normalisierte Eingabe ohne Datenbank. Ob fuer
// "aktiv" ein Passwort vorliegt, entscheidet Save (gespeichertes zaehlt).
// Zeilenumbrueche sind ueberall verboten - sie oeffneten Kopfzeilen und
// SMTP-Befehle fuer Einschleusungen.
func ValidateInput(in Input) error {
	if in.Sicherheit != SicherheitSSL && in.Sicherheit != SicherheitStartTLS {
		return errors.New("Ungültige Verschlüsselung (ssl oder starttls)")
	}
	if in.SMTPPort < 1 || in.SMTPPort > 65535 {
		return errors.New("Port muss zwischen 1 und 65535 liegen")
	}
	if in.SMTPHost != "" && (len(in.SMTPHost) > 253 || strings.ContainsAny(in.SMTPHost, " \t\r\n/:@")) {
		return errors.New("Mail-Server ungültig (nur der Name, z. B. mail.example.de)")
	}
	if hasLineBreak(in.Benutzer) || len(in.Benutzer) > 320 {
		return errors.New("Benutzer ungültig")
	}
	if hasLineBreak(in.Passwort) || len(in.Passwort) > 512 {
		return errors.New("Passwort ungültig")
	}
	if in.Passwort != "" && in.PasswortLoeschen {
		return errors.New("Passwort entweder neu setzen oder löschen")
	}
	if in.AbsenderAdresse != "" {
		a, err := netmail.ParseAddress(in.AbsenderAdresse)
		if err != nil || a.Address != in.AbsenderAdresse || hasLineBreak(in.AbsenderAdresse) {
			return errors.New("Absenderadresse ungültig")
		}
	}
	if hasLineBreak(in.AbsenderName) || len(in.AbsenderName) > 200 {
		return errors.New("Absendername ungültig")
	}
	if in.Aktiv {
		missing := make([]string, 0)
		if in.SMTPHost == "" {
			missing = append(missing, "Mail-Server")
		}
		if in.Benutzer == "" {
			missing = append(missing, "Benutzer")
		}
		if in.AbsenderAdresse == "" {
			missing = append(missing, "Absenderadresse")
		}
		if len(missing) > 0 {
			return errors.New("Für den Versand fehlt: " + strings.Join(missing, ", "))
		}
	}
	return nil
}

type SettingsService struct {
	pg  *pgxpool.Pool
	box *secretbox.Box
}

// NewSettingsService: box nil = kein Schluessel eingerichtet.
func NewSettingsService(pg *pgxpool.Pool, box *secretbox.Box) *SettingsService {
	return &SettingsService{pg: pg, box: box}
}

type row struct {
	s   Settings
	enc []byte
}

func (s *SettingsService) load(ctx context.Context, q pgx.Row) (*row, error) {
	var r row
	err := q.Scan(&r.s.Aktiv, &r.s.SMTPHost, &r.s.SMTPPort, &r.s.Sicherheit, &r.s.Benutzer, &r.enc,
		&r.s.AbsenderAdresse, &r.s.AbsenderName, &r.s.UpdatedByUserID, &r.s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &row{s: Settings{SMTPPort: defaultPorts[SicherheitSSL], Sicherheit: SicherheitSSL}}, nil
	}
	if err != nil {
		return nil, err
	}
	r.s.Eingerichtet = true
	return &r, nil
}

const settingsSelect = `SELECT aktiv, smtp_host, smtp_port, sicherheit, benutzer, password_enc,
	absender_adresse, absender_name, updated_by_user_id, updated_at
	FROM company_mail_settings WHERE company_id=$1`

func (s *SettingsService) view(r *row, companyID string) *Settings {
	out := r.s
	out.SchluesselEingerichtet = s.box != nil
	out.PasswortGesetzt = r.enc != nil
	if r.enc != nil {
		_, err := s.box.Open(r.enc, passwordAAD(companyID))
		out.PasswortLesbar = err == nil
	}
	return &out
}

// Get liefert die Einstellung des Mandanten; ohne gespeicherte die
// Standardwerte (Eingerichtet=false).
func (s *SettingsService) Get(ctx context.Context, companyID string) (*Settings, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	r, err := s.load(ctx, s.pg.QueryRow(ctx, settingsSelect, companyID))
	if err != nil {
		return nil, err
	}
	return s.view(r, companyID), nil
}

// Save ersetzt die Einstellung des Mandanten.
func (s *SettingsService) Save(ctx context.Context, in Input, actorUserID, companyID string) (*Settings, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in = NormalizeInput(in)
	if err := ValidateInput(in); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cur, err := s.load(ctx, tx.QueryRow(ctx, settingsSelect+` FOR UPDATE`, companyID))
	if err != nil {
		return nil, err
	}
	enc := cur.enc
	switch {
	case in.Passwort != "":
		if enc, err = s.box.Seal([]byte(in.Passwort), passwordAAD(companyID)); err != nil {
			return nil, err
		}
	case in.PasswortLoeschen:
		enc = nil
	}
	if in.Aktiv && enc == nil {
		return nil, errors.New("Für den Versand fehlt: Passwort")
	}
	var actor *string
	if strings.TrimSpace(actorUserID) != "" {
		actor = &actorUserID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO company_mail_settings (company_id, aktiv, smtp_host, smtp_port, sicherheit, benutzer, password_enc,
			absender_adresse, absender_name, updated_by_user_id, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now())
		ON CONFLICT (company_id) DO UPDATE SET
			aktiv=EXCLUDED.aktiv, smtp_host=EXCLUDED.smtp_host, smtp_port=EXCLUDED.smtp_port,
			sicherheit=EXCLUDED.sicherheit, benutzer=EXCLUDED.benutzer, password_enc=EXCLUDED.password_enc,
			absender_adresse=EXCLUDED.absender_adresse, absender_name=EXCLUDED.absender_name,
			updated_by_user_id=EXCLUDED.updated_by_user_id, updated_at=now()`,
		companyID, in.Aktiv, in.SMTPHost, in.SMTPPort, in.Sicherheit, in.Benutzer, enc,
		in.AbsenderAdresse, in.AbsenderName, actor); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, companyID)
}

// Config liefert den entschluesselten Zugang fuer den Versand - auch
// inaktiv, damit sich ein Zugang vor dem Einschalten testen laesst.
func (s *SettingsService) Config(ctx context.Context, companyID string) (*SMTPConfig, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	r, err := s.load(ctx, s.pg.QueryRow(ctx, settingsSelect, companyID))
	if err != nil {
		return nil, err
	}
	if !r.s.Eingerichtet || r.s.SMTPHost == "" || r.s.Benutzer == "" || r.s.AbsenderAdresse == "" || r.enc == nil {
		return nil, errors.New("E-Mail-Versand ist nicht vollständig eingerichtet")
	}
	pw, err := s.box.Open(r.enc, passwordAAD(companyID))
	if err != nil {
		return nil, err
	}
	return &SMTPConfig{Aktiv: r.s.Aktiv, Host: r.s.SMTPHost, Port: r.s.SMTPPort, Sicherheit: r.s.Sicherheit, Benutzer: r.s.Benutzer,
		Passwort: string(pw), AbsenderAdresse: r.s.AbsenderAdresse, AbsenderName: r.s.AbsenderName}, nil
}
