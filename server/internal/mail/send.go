package mail

// SMTP-Versand (Backlog F.14.2b): nur verschluesselt - "ssl" (TLS ab dem
// ersten Byte, ueblich Port 465) oder "starttls" (Port 587, ohne
// angebotenes STARTTLS kein Versand, nie Rueckfall auf Klartext).
//
// Fehler nennen nur, woran es lag (Verbindung, Verschluesselung,
// Anmeldung, Empfaenger ...). Den Antworttext des Mail-Servers geben sie
// nicht weiter: er kann Benutzer, Adressen oder Teile des Passworts
// enthalten und landet sonst in Logs und Meldungen.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Hoechstzahl Empfaenger je Nachricht.
const maxRecipients = 50

// Message ist eine Textnachricht. Der Text ist UTF-8; Zeilenenden werden
// beim Versand zu CRLF.
type Message struct {
	To      []string
	Subject string
	Text    string
}

// Stufen, an denen ein Versand scheitern kann.
const (
	StageVerbindung = "verbindung"
	StageTLS        = "tls"
	StageAnmeldung  = "anmeldung"
	StageAbsender   = "absender"
	StageEmpfaenger = "empfaenger"
	StageNachricht  = "nachricht"
)

// SendError ist ein zusammengefasster Versandfehler - ohne Antworttext
// des Mail-Servers und ohne Ursache, die ihn enthalten koennte.
type SendError struct {
	Stage string
	msg   string
}

func (e *SendError) Error() string { return e.msg }

func sendErr(stage, msg string) error { return &SendError{Stage: stage, msg: msg} }

// Sender verschickt Nachrichten ueber den Zugang eines Mandanten.
type Sender struct {
	// TLSConfig ist die Vorlage fuer die Verbindung (nil: Systemzertifikate).
	// ServerName wird auf den Mail-Server gesetzt, wenn leer.
	TLSConfig *tls.Config
	// Timeout fuer den ganzen Versand (0: 30 Sekunden).
	Timeout time.Duration
	// Now fuer die Date-Kopfzeile (nil: time.Now).
	Now func() time.Time
}

// BuildMessage erzeugt die Nachricht mit Kopfzeilen (RFC 5322, UTF-8,
// quoted-printable). Empfaenger und Betreff werden vorher geprueft.
func BuildMessage(cfg SMTPConfig, msg Message, now time.Time) ([]byte, error) {
	if err := validateMessage(cfg, msg); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", (&netmail.Address{Name: cfg.AbsenderName, Address: cfg.AbsenderAdresse}).String())
	header("To", strings.Join(msg.To, ", "))
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", messageID(cfg.AbsenderAdresse))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	// quoted-printable macht aus \n und \r\n jeweils CRLF.
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(msg.Text)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func messageID(from string) string {
	domain := "localhost"
	if i := strings.LastIndexByte(from, '@'); i >= 0 {
		domain = from[i+1:]
	}
	r := make([]byte, 16)
	_, _ = rand.Read(r)
	return "<" + hex.EncodeToString(r) + "@" + domain + ">"
}

func validateMessage(cfg SMTPConfig, msg Message) error {
	if cfg.Host == "" || cfg.Benutzer == "" || cfg.Passwort == "" || cfg.AbsenderAdresse == "" || cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("E-Mail-Versand ist nicht vollständig eingerichtet")
	}
	if cfg.Sicherheit != SicherheitSSL && cfg.Sicherheit != SicherheitStartTLS {
		return errors.New("Ungültige Verschlüsselung (ssl oder starttls)")
	}
	if a, err := netmail.ParseAddress(cfg.AbsenderAdresse); err != nil || a.Address != cfg.AbsenderAdresse {
		return errors.New("Absenderadresse ungültig")
	}
	if hasLineBreak(cfg.AbsenderName) || hasLineBreak(cfg.Benutzer) || hasLineBreak(cfg.Passwort) {
		return errors.New("E-Mail-Zugang ungültig")
	}
	if len(msg.To) == 0 {
		return errors.New("Mindestens ein Empfänger erforderlich")
	}
	if len(msg.To) > maxRecipients {
		return fmt.Errorf("Höchstens %d Empfänger je Nachricht", maxRecipients)
	}
	for _, to := range msg.To {
		if a, err := netmail.ParseAddress(to); err != nil || a.Address != to || hasLineBreak(to) {
			return errors.New("Empfängeradresse ungültig")
		}
	}
	if strings.TrimSpace(msg.Subject) == "" || hasLineBreak(msg.Subject) || len(msg.Subject) > 200 {
		return errors.New("Betreff ungültig")
	}
	if strings.TrimSpace(msg.Text) == "" {
		return errors.New("Text erforderlich")
	}
	return nil
}

// Send verschickt msg ueber cfg. Eingaben werden vor dem Verbinden
// geprueft; Fehler danach sind *SendError.
func (s *Sender) Send(ctx context.Context, cfg SMTPConfig, msg Message) error {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	data, err := BuildMessage(cfg, msg, now())
	if err != nil {
		return err
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return classify(ctx, StageVerbindung, err, "Mail-Server nicht erreichbar")
	}
	defer conn.Close()
	// Zeitlimit oder Abbruch schliessen die Verbindung; jeder wartende
	// Lese- oder Schreibvorgang endet dann sofort.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	tlsCfg := s.tlsConfig(cfg.Host)
	if cfg.Sicherheit == SicherheitSSL {
		tc := tls.Client(conn, tlsCfg)
		if err := tc.HandshakeContext(ctx); err != nil {
			return classify(ctx, StageTLS, err, "Verschlüsselte Verbindung zum Mail-Server fehlgeschlagen")
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return classify(ctx, StageVerbindung, err, "Mail-Server antwortet nicht wie erwartet")
	}
	defer c.Close()
	if err := c.Hello("localhost"); err != nil {
		return classify(ctx, StageVerbindung, err, "Mail-Server antwortet nicht wie erwartet")
	}
	if cfg.Sicherheit == SicherheitStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return sendErr(StageTLS, "Mail-Server bietet keine verschlüsselte Verbindung (STARTTLS) an")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return classify(ctx, StageTLS, err, "Verschlüsselte Verbindung zum Mail-Server fehlgeschlagen")
		}
	}
	auth, err := chooseAuth(c, cfg)
	if err != nil {
		return err
	}
	if err := c.Auth(auth); err != nil {
		return classify(ctx, StageAnmeldung, err, "Anmeldung am Mail-Server fehlgeschlagen (Benutzer oder Passwort prüfen)")
	}
	if err := c.Mail(cfg.AbsenderAdresse); err != nil {
		return classify(ctx, StageAbsender, err, "Mail-Server hat die Absenderadresse abgelehnt")
	}
	for _, to := range msg.To {
		if err := c.Rcpt(to); err != nil {
			return classify(ctx, StageEmpfaenger, err, "Mail-Server hat einen Empfänger abgelehnt")
		}
	}
	w, err := c.Data()
	if err != nil {
		return classify(ctx, StageNachricht, err, "Mail-Server hat die Nachricht abgelehnt")
	}
	if _, err := w.Write(data); err != nil {
		return classify(ctx, StageNachricht, err, "Mail-Server hat die Nachricht abgelehnt")
	}
	if err := w.Close(); err != nil {
		return classify(ctx, StageNachricht, err, "Mail-Server hat die Nachricht abgelehnt")
	}
	// Die Nachricht ist angenommen; ein Fehler beim Abmelden aendert daran nichts.
	_ = c.Quit()
	return nil
}

func (s *Sender) tlsConfig(host string) *tls.Config {
	var c *tls.Config
	if s.TLSConfig != nil {
		c = s.TLSConfig.Clone()
	} else {
		c = &tls.Config{}
	}
	if c.ServerName == "" {
		c.ServerName = host
	}
	if c.MinVersion < tls.VersionTLS12 {
		c.MinVersion = tls.VersionTLS12
	}
	return c
}

// classify fasst err zusammen; Zeitueberschreitung und Abbruch werden
// eigens genannt (bei beiden wird die Verbindung geschlossen, err selbst
// sagt dann nur "closed").
func classify(ctx context.Context, stage string, err error, msg string) error {
	var ne net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return sendErr(stage, "Zeitüberschreitung beim Mail-Server")
	}
	if ctx.Err() != nil {
		return sendErr(stage, "Versand abgebrochen")
	}
	return sendErr(stage, msg)
}

// chooseAuth nimmt PLAIN, sonst LOGIN - beide nur ueber TLS.
func chooseAuth(c *smtp.Client, cfg SMTPConfig) (smtp.Auth, error) {
	ok, params := c.Extension("AUTH")
	if !ok {
		return nil, sendErr(StageAnmeldung, "Mail-Server bietet keine Anmeldung an")
	}
	mechs := strings.Fields(strings.ToUpper(params))
	for _, m := range mechs {
		if m == "PLAIN" {
			return smtp.PlainAuth("", cfg.Benutzer, cfg.Passwort, cfg.Host), nil
		}
	}
	for _, m := range mechs {
		if m == "LOGIN" {
			return &loginAuth{user: cfg.Benutzer, pass: cfg.Passwort}, nil
		}
	}
	return nil, sendErr(StageAnmeldung, "Mail-Server bietet kein unterstütztes Anmeldeverfahren an (PLAIN oder LOGIN)")
}

// loginAuth: AUTH LOGIN, das net/smtp nicht mitbringt.
type loginAuth struct{ user, pass string }

func (a *loginAuth) Start(si *smtp.ServerInfo) (string, []byte, error) {
	if !si.TLS {
		return "", nil, errors.New("unverschlüsselte Verbindung")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(from []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(from))) {
	case "username:":
		return []byte(a.user), nil
	case "password:":
		return []byte(a.pass), nil
	}
	return nil, errors.New("unerwartete Anfrage bei der Anmeldung")
}
