package mail

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	netmail "net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// SMTP-Versand gegen einen Test-Mailserver im Testprozess (Backlog
// F.14.2b). Erfundene Zugangsdaten; der echte Versand laeuft nur mit
// NALA_SMTP_LIVE=1 gegen DEV_SMTP_* aus der lokalen Umgebung.

const (
	fakeUser = "erp@example.test"
	fakePass = "erfundenes-Passwort-42"
)

// fakeSMTP ist ein kleiner SMTP-Server: EHLO, STARTTLS, AUTH PLAIN/LOGIN,
// MAIL, RCPT, DATA, QUIT.
type fakeSMTP struct {
	t           *testing.T
	ln          net.Listener
	tlsCfg      *tls.Config
	implicitTLS bool   // TLS ab dem ersten Byte
	startTLS    bool   // STARTTLS anbieten
	authMechs   string // "" = kein AUTH
	rejectRcpt  string
	rejectData  bool // lehnt die Nachricht nach DATA ab
	silent      bool // nimmt an, schweigt

	mu       sync.Mutex
	authTry  int
	mech     string // zuletzt benutztes Anmeldeverfahren
	mailCmds int
	got      []fakeMail
}

type fakeMail struct {
	from   string
	to     []string
	data   string
	tls    bool
	authed bool
}

func testCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

// startFake startet den Server; der Sender vertraut dessen Zertifikat.
func startFake(t *testing.T, setup func(*fakeSMTP)) (*fakeSMTP, SMTPConfig, *Sender) {
	t.Helper()
	cert, pool := testCert(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{t: t, ln: ln, tlsCfg: &tls.Config{Certificates: []tls.Certificate{cert}}, authMechs: "PLAIN LOGIN"}
	setup(f)
	go f.serve()
	t.Cleanup(func() { ln.Close() })
	sec := SicherheitStartTLS
	if f.implicitTLS {
		sec = SicherheitSSL
	}
	port, _ := strconv.Atoi(strings.Split(ln.Addr().String(), ":")[1])
	cfg := SMTPConfig{Aktiv: true, Host: "127.0.0.1", Port: port, Sicherheit: sec, Benutzer: fakeUser, Passwort: fakePass,
		AbsenderAdresse: "erp@example.test", AbsenderName: "Nala ERP"}
	return f, cfg, &Sender{TLSConfig: &tls.Config{RootCAs: pool}, Timeout: 5 * time.Second}
}

func (f *fakeSMTP) serve() {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(c)
	}
}

func (f *fakeSMTP) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if f.silent {
		_, _ = io.Copy(io.Discard, conn)
		return
	}
	isTLS := false
	if f.implicitTLS {
		tc := tls.Server(conn, f.tlsCfg)
		if tc.Handshake() != nil {
			return
		}
		conn, isTLS = tc, true
	}
	r := bufio.NewReader(conn)
	say := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
	read := func() string { l, _ := r.ReadString('\n'); return strings.TrimRight(l, "\r\n") }
	say("220 test ESMTP")
	cur := fakeMail{}
	authed := false
	for {
		line := read()
		cmd := strings.ToUpper(line)
		switch {
		case line == "":
			return
		case strings.HasPrefix(cmd, "EHLO"):
			say("250-test")
			if f.startTLS && !isTLS {
				say("250-STARTTLS")
			}
			if f.authMechs != "" {
				say("250-AUTH " + f.authMechs)
			}
			say("250 8BITMIME")
		case cmd == "STARTTLS":
			say("220 go ahead")
			tc := tls.Server(conn, f.tlsCfg)
			if tc.Handshake() != nil {
				return
			}
			conn, isTLS = tc, true
			r = bufio.NewReader(conn)
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			f.count(&f.authTry, "PLAIN")
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(line[len("AUTH PLAIN"):]))
			authed = string(raw) == "\x00"+fakeUser+"\x00"+fakePass
			f.authReply(say, authed)
		case cmd == "AUTH LOGIN":
			f.count(&f.authTry, "LOGIN")
			say("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
			u, _ := base64.StdEncoding.DecodeString(read())
			say("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
			p, _ := base64.StdEncoding.DecodeString(read())
			authed = string(u) == fakeUser && string(p) == fakePass
			f.authReply(say, authed)
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			f.count(&f.mailCmds, "")
			if !authed {
				say("530 5.7.0 Authentication required")
				continue
			}
			cur = fakeMail{from: between(line), tls: isTLS, authed: authed}
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			to := between(line)
			if to == f.rejectRcpt {
				say("550 5.1.1 <" + to + "> unknown user")
				continue
			}
			cur.to = append(cur.to, to)
			say("250 ok")
		case cmd == "DATA":
			say("354 go")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(strings.TrimPrefix(l, "."))
			}
			if f.rejectData {
				say("554 5.7.1 message rejected")
				continue
			}
			cur.data = b.String()
			f.mu.Lock()
			f.got = append(f.got, cur)
			f.mu.Unlock()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("250 ok")
		}
	}
}

// Die Ablehnung nennt Benutzer und Passwort - der Sender darf sie nicht weitergeben.
func (f *fakeSMTP) authReply(say func(string), ok bool) {
	if ok {
		say("235 ok")
		return
	}
	say("535 5.7.8 Authentication failed for " + fakeUser + " with " + fakePass)
}

func (f *fakeSMTP) count(n *int, mech string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	*n++
	if mech != "" {
		f.mech = mech
	}
}

func (f *fakeSMTP) mails() []fakeMail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeMail(nil), f.got...)
}

func between(line string) string {
	i, j := strings.IndexByte(line, '<'), strings.IndexByte(line, '>')
	if i < 0 || j < i {
		return ""
	}
	return line[i+1 : j]
}

var testMsg = Message{To: []string{"empfaenger@example.test"}, Subject: "Fällige Nachweise", Text: "Grüße\n.punkt am Zeilenanfang\nEnde"}

// decode zerlegt eine empfangene Nachricht und dekodiert Betreff und Text.
func decode(t *testing.T, data string) (*netmail.Message, string, string) {
	t.Helper()
	m, err := netmail.ReadMessage(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Nachricht: %v", err)
	}
	subj, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("Betreff: %v", err)
	}
	body, err := io.ReadAll(quotedprintable.NewReader(m.Body))
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	return m, subj, string(body)
}

func TestBuildMessage(t *testing.T) {
	cfg := SMTPConfig{Host: "mail.example.test", Port: 465, Sicherheit: SicherheitSSL, Benutzer: fakeUser, Passwort: fakePass, AbsenderAdresse: "erp@example.test", AbsenderName: "Büro Nala"}
	now := time.Date(2026, 10, 29, 7, 0, 0, 0, time.FixedZone("CET", 3600))
	raw, err := BuildMessage(cfg, Message{To: []string{"a@example.test", "b@example.test"}, Subject: "Fällige Nachweise", Text: "Zeile 1\r\nZeile ä\n"}, now)
	if err != nil {
		t.Fatalf("bauen: %v", err)
	}
	s := string(raw)
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") || strings.Contains(s, fakePass) {
		t.Fatalf("Zeilenenden oder Passwort: %q", s)
	}
	m, subj, body := decode(t, s)
	from, err := m.Header.AddressList("From")
	if err != nil || len(from) != 1 || from[0].Name != "Büro Nala" || from[0].Address != "erp@example.test" {
		t.Fatalf("From: %v %v", from, err)
	}
	if to, _ := m.Header.AddressList("To"); len(to) != 2 || to[1].Address != "b@example.test" {
		t.Fatalf("To: %v", to)
	}
	if subj != "Fällige Nachweise" || !strings.HasPrefix(m.Header.Get("Subject"), "=?utf-8?") {
		t.Fatalf("Betreff: %q / %q", subj, m.Header.Get("Subject"))
	}
	if body != "Zeile 1\r\nZeile ä\r\n" {
		t.Fatalf("Text: %q", body)
	}
	if d, _ := m.Header.Date(); !d.Equal(now) {
		t.Fatalf("Date: %v", d)
	}
	if id := m.Header.Get("Message-ID"); !strings.HasSuffix(id, "@example.test>") || len(id) != len("<@example.test>")+32 {
		t.Fatalf("Message-ID: %q", id)
	}
	if m.Header.Get("Content-Type") != "text/plain; charset=utf-8" || m.Header.Get("MIME-Version") != "1.0" {
		t.Fatalf("MIME: %v", m.Header)
	}
	// Ohne Absendername nur die Adresse.
	cfg.AbsenderName = ""
	raw, _ = BuildMessage(cfg, testMsg, now)
	if !strings.HasPrefix(string(raw), "From: <erp@example.test>\r\n") {
		t.Fatalf("From ohne Namen: %q", raw)
	}

	many := make([]string, 51)
	for i := range many {
		many[i] = "x" + strconv.Itoa(i) + "@example.test"
	}
	bad := cfg
	bad.Sicherheit = "none"
	noPass := cfg
	noPass.Passwort = ""
	nameBreak := cfg
	nameBreak.AbsenderName = "ERP\r\nBcc: x@example.test"
	for name, tc := range map[string]struct {
		cfg  SMTPConfig
		msg  Message
		want string
	}{
		"ohne Passwort":       {noPass, testMsg, "E-Mail-Versand ist nicht vollständig eingerichtet"},
		"Verschluesselung":    {bad, testMsg, "Ungültige Verschlüsselung (ssl oder starttls)"},
		"Absendername":        {nameBreak, testMsg, "E-Mail-Zugang ungültig"},
		"ohne Empfaenger":     {cfg, Message{Subject: "x", Text: "x"}, "Mindestens ein Empfänger erforderlich"},
		"zu viele":            {cfg, Message{To: many, Subject: "x", Text: "x"}, "Höchstens 50 Empfänger je Nachricht"},
		"Empfaenger Umbruch":  {cfg, Message{To: []string{"a@example.test\r\nRCPT TO:<b@example.test>"}, Subject: "x", Text: "x"}, "Empfängeradresse ungültig"},
		"Empfaenger mit Name": {cfg, Message{To: []string{"A <a@example.test>"}, Subject: "x", Text: "x"}, "Empfängeradresse ungültig"},
		"Empfaenger Liste":    {cfg, Message{To: []string{"a@example.test, b@example.test"}, Subject: "x", Text: "x"}, "Empfängeradresse ungültig"},
		"Betreff Umbruch":     {cfg, Message{To: []string{"a@example.test"}, Subject: "x\r\nBcc: b@example.test", Text: "x"}, "Betreff ungültig"},
		"Betreff leer":        {cfg, Message{To: []string{"a@example.test"}, Subject: " ", Text: "x"}, "Betreff ungültig"},
		"Text leer":           {cfg, Message{To: []string{"a@example.test"}, Subject: "x", Text: "\n"}, "Text erforderlich"},
	} {
		if _, err := BuildMessage(tc.cfg, tc.msg, now); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := BuildMessage(cfg, Message{To: many[:50], Subject: "x", Text: "x"}, now); err != nil {
		t.Fatalf("50 Empfaenger: %v", err)
	}
}

func TestSendDelivers(t *testing.T) {
	for name, tc := range map[string]struct {
		setup func(*fakeSMTP)
		mech  string
	}{
		"ssl":            {func(f *fakeSMTP) { f.implicitTLS = true }, "PLAIN"},
		"starttls plain": {func(f *fakeSMTP) { f.startTLS = true }, "PLAIN"},
		"starttls login": {func(f *fakeSMTP) { f.startTLS, f.authMechs = true, "LOGIN" }, "LOGIN"},
	} {
		f, cfg, s := startFake(t, tc.setup)
		s.Timeout = 0 // Standard-Zeitlimit
		msg := testMsg
		msg.To = []string{"a@example.test", "b@example.test"}
		if err := s.Send(context.Background(), cfg, msg); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := f.mails()
		if f.mech != tc.mech {
			t.Fatalf("%s: Anmeldung per %q", name, f.mech)
		}
		if len(got) != 1 || !got[0].tls || !got[0].authed || got[0].from != "erp@example.test" || strings.Join(got[0].to, ",") != "a@example.test,b@example.test" {
			t.Fatalf("%s: %+v", name, got)
		}
		_, subj, body := decode(t, got[0].data)
		if subj != "Fällige Nachweise" || body != "Grüße\r\n.punkt am Zeilenanfang\r\nEnde\r\n" {
			t.Fatalf("%s: %q %q", name, subj, body)
		}
	}
}

func TestSendFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		setup  func(*fakeSMTP)
		change func(*SMTPConfig, *Sender)
		stage  string
		want   string
		auth   bool // Anmeldung versucht
	}{
		"ohne STARTTLS": {func(f *fakeSMTP) {}, nil, StageTLS, "Mail-Server bietet keine verschlüsselte Verbindung (STARTTLS) an", false},
		"fremdes Zertifikat": {func(f *fakeSMTP) { f.implicitTLS = true },
			func(_ *SMTPConfig, s *Sender) { s.TLSConfig = nil }, StageTLS, "Verschlüsselte Verbindung zum Mail-Server fehlgeschlagen", false},
		"ssl gegen Klartext": {func(f *fakeSMTP) { f.startTLS = true },
			func(c *SMTPConfig, _ *Sender) { c.Sicherheit = SicherheitSSL }, StageTLS, "Verschlüsselte Verbindung zum Mail-Server fehlgeschlagen", false},
		"falsches Passwort": {func(f *fakeSMTP) { f.implicitTLS = true },
			func(c *SMTPConfig, _ *Sender) { c.Passwort = "falsch" }, StageAnmeldung, "Anmeldung am Mail-Server fehlgeschlagen (Benutzer oder Passwort prüfen)", true},
		"falsches Passwort LOGIN": {func(f *fakeSMTP) { f.implicitTLS, f.authMechs = true, "LOGIN" },
			func(c *SMTPConfig, _ *Sender) { c.Passwort = "falsch" }, StageAnmeldung, "Anmeldung am Mail-Server fehlgeschlagen (Benutzer oder Passwort prüfen)", true},
		"kein AUTH": {func(f *fakeSMTP) { f.implicitTLS, f.authMechs = true, "" }, nil, StageAnmeldung, "Mail-Server bietet keine Anmeldung an", false},
		"nur CRAM-MD5": {func(f *fakeSMTP) { f.implicitTLS, f.authMechs = true, "CRAM-MD5" }, nil, StageAnmeldung,
			"Mail-Server bietet kein unterstütztes Anmeldeverfahren an (PLAIN oder LOGIN)", false},
		"Empfaenger abgelehnt": {func(f *fakeSMTP) { f.implicitTLS, f.rejectRcpt = true, "empfaenger@example.test" }, nil, StageEmpfaenger, "Mail-Server hat einen Empfänger abgelehnt", true},
		"Nachricht abgelehnt":  {func(f *fakeSMTP) { f.implicitTLS, f.rejectData = true, true }, nil, StageNachricht, "Mail-Server hat die Nachricht abgelehnt", true},
		"schweigt": {func(f *fakeSMTP) { f.silent = true },
			func(_ *SMTPConfig, s *Sender) { s.Timeout = 300 * time.Millisecond }, StageVerbindung, "Zeitüberschreitung beim Mail-Server", false},
	} {
		f, cfg, s := startFake(t, tc.setup)
		if tc.change != nil {
			tc.change(&cfg, s)
		}
		start := time.Now()
		err := s.Send(context.Background(), cfg, testMsg)
		var se *SendError
		if !errors.As(err, &se) || se.Stage != tc.stage || err.Error() != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
		if time.Since(start) > 4*time.Second {
			t.Fatalf("%s: dauerte %v", name, time.Since(start))
		}
		f.mu.Lock()
		authTry, mailCmds := f.authTry, f.mailCmds
		f.mu.Unlock()
		if len(f.mails()) != 0 || (authTry > 0) != tc.auth || (!tc.auth && mailCmds > 0) {
			t.Fatalf("%s: zugestellt %d, Anmeldung %d, MAIL %d", name, len(f.mails()), authTry, mailCmds)
		}
	}
}

// Mit RedirectTo geht jede Nachricht nur an die Umleitung; ungueltige
// Nachrichten werden trotzdem abgelehnt.
func TestSendRedirect(t *testing.T) {
	f, cfg, s := startFake(t, func(f *fakeSMTP) { f.implicitTLS = true })
	s.RedirectTo = "umleitung@example.test"
	msg := testMsg
	msg.To = []string{"a@example.test", "b@example.test"}
	if err := s.Send(context.Background(), cfg, msg); err != nil {
		t.Fatalf("umgeleitet: %v", err)
	}
	got := f.mails()
	if len(got) != 1 || strings.Join(got[0].to, ",") != "umleitung@example.test" {
		t.Fatalf("Empfaenger: %+v", got)
	}
	m, subj, _ := decode(t, got[0].data)
	if subj != "[Umgeleitet] Fällige Nachweise" || m.Header.Get("To") != "umleitung@example.test" {
		t.Fatalf("Kopf: %q %q", subj, m.Header.Get("To"))
	}
	bad := testMsg
	bad.To = []string{"a@example.test\r\nRCPT TO:<b@example.test>"}
	if err := s.Send(context.Background(), cfg, bad); err == nil || err.Error() != "Empfängeradresse ungültig" {
		t.Fatalf("ungueltig umgeleitet: %v", err)
	}
	if len(f.mails()) != 1 {
		t.Fatal("ungueltige Nachricht umgeleitet zugestellt")
	}
}

// AUTH LOGIN gibt Benutzer und Passwort nie ueber eine unverschluesselte Verbindung.
func TestLoginAuthRequiresTLS(t *testing.T) {
	a := &loginAuth{user: fakeUser, pass: fakePass}
	if _, _, err := a.Start(&smtp.ServerInfo{Name: "mail.example.test"}); err == nil {
		t.Fatal("ohne TLS angenommen")
	}
	if mech, _, err := a.Start(&smtp.ServerInfo{Name: "mail.example.test", TLS: true}); err != nil || mech != "LOGIN" {
		t.Fatalf("mit TLS: %q %v", mech, err)
	}
	if _, err := a.Next([]byte("Wer bist du?"), true); err == nil {
		t.Fatal("unerwartete Anfrage beantwortet")
	}
}

// Die Antwort des Mail-Servers (mit Benutzer und Passwort) erscheint nicht im Fehler.
func TestSendErrorHidesServerReply(t *testing.T) {
	_, cfg, s := startFake(t, func(f *fakeSMTP) { f.implicitTLS = true })
	cfg.Benutzer = "anderer@example.test"
	err := s.Send(context.Background(), cfg, testMsg)
	if err == nil {
		t.Fatal("angenommen")
	}
	for _, leak := range []string{fakePass, fakeUser, "anderer@", "535", "Authentication"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("Fehler enthaelt %q: %v", leak, err)
		}
	}
	if errors.Unwrap(err) != nil {
		t.Fatal("Fehler traegt eine Ursache")
	}
}

func TestSendUnreachableAndInvalid(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port, _ := strconv.Atoi(strings.Split(ln.Addr().String(), ":")[1])
	ln.Close()
	cfg := SMTPConfig{Host: "127.0.0.1", Port: port, Sicherheit: SicherheitSSL, Benutzer: fakeUser, Passwort: fakePass, AbsenderAdresse: "erp@example.test"}
	s := &Sender{Timeout: 2 * time.Second}
	var se *SendError
	if err := s.Send(context.Background(), cfg, testMsg); !errors.As(err, &se) || se.Stage != StageVerbindung || err.Error() != "Mail-Server nicht erreichbar" {
		t.Fatalf("nicht erreichbar: %v", err)
	}
	// Ungueltige Nachricht: abgelehnt, ohne zu verbinden (kein SendError).
	if err := s.Send(context.Background(), cfg, Message{Subject: "x", Text: "x"}); err == nil || errors.As(err, &se) {
		t.Fatalf("ungueltig: %v", err)
	}
	// Abgebrochener Kontext.
	_, cfg2, s2 := startFake(t, func(f *fakeSMTP) { f.silent = true })
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	start := time.Now()
	if err := s2.Send(ctx, cfg2, testMsg); !errors.As(err, &se) || err.Error() != "Versand abgebrochen" || time.Since(start) > 3*time.Second {
		t.Fatalf("abgebrochen: %v nach %v", err, time.Since(start))
	}
}

// Echter Versand an das Entwicklungskonto: nur mit NALA_SMTP_LIVE=1; die
// Zugangsdaten kommen aus der Umgebung (lokale .env), nie aus dem Repo.
func TestSendLive(t *testing.T) {
	if os.Getenv("NALA_SMTP_LIVE") != "1" {
		t.Skip("NALA_SMTP_LIVE=1 fuer echten Versand")
	}
	port, _ := strconv.Atoi(os.Getenv("DEV_SMTP_PORT"))
	cfg := SMTPConfig{Host: os.Getenv("DEV_SMTP_HOST"), Port: port, Sicherheit: os.Getenv("DEV_SMTP_SICHERHEIT"),
		Benutzer: os.Getenv("DEV_SMTP_USER"), Passwort: os.Getenv("DEV_SMTP_PASSWORD"), AbsenderAdresse: os.Getenv("DEV_SMTP_FROM"), AbsenderName: "Nala ERP Test"}
	// Empfaenger nur die vom Nutzer vorgegebene Testadresse (DEV_SMTP_TEST_TO).
	to := os.Getenv("DEV_SMTP_TEST_TO")
	if to == "" {
		t.Fatal("DEV_SMTP_TEST_TO fehlt - Testmails nur an die vorgegebene Testadresse")
	}
	err := (&Sender{}).Send(context.Background(), cfg, Message{To: []string{to}, Subject: "Nala ERP: Testversand äöü", Text: "Testnachricht aus TestSendLive.\nUmlaute: äöüß"})
	if err != nil {
		t.Fatalf("Versand: %v", err)
	}
}
