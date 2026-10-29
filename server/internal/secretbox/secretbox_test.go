package secretbox

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// Verschluesselte Einstellungen (ADR 0033, Backlog F.14.1). Schluessel und
// Klartexte sind erfundene Testwerte.

func testKey(fill byte) []byte { return bytes.Repeat([]byte{fill}, 32) }

func TestParseKey(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(testKey(7))
	if k, err := ParseKey("  " + good + "\n"); err != nil || !bytes.Equal(k, testKey(7)) {
		t.Fatalf("gueltig: %v", err)
	}
	for name, tc := range map[string][2]string{
		"kein Base64": {"%%%", "SETTINGS_ENCRYPTION_KEY ist kein gültiges Base64"},
		"zu kurz":     {base64.StdEncoding.EncodeToString(make([]byte, 16)), "SETTINGS_ENCRYPTION_KEY muss 32 Byte lang sein (z. B. 'openssl rand -base64 32')"},
		"zu lang":     {base64.StdEncoding.EncodeToString(make([]byte, 33)), "SETTINGS_ENCRYPTION_KEY muss 32 Byte lang sein (z. B. 'openssl rand -base64 32')"},
	} {
		if _, err := ParseKey(tc[0]); err == nil || err.Error() != tc[1] {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Nur AES-256: 16 und 24 Byte nimmt AES selbst an, New nicht.
	for _, n := range []int{16, 24, 31, 33} {
		if _, err := New(make([]byte, n)); err == nil {
			t.Fatalf("New mit %d Byte angenommen", n)
		}
	}
}

func TestFromEnvValue(t *testing.T) {
	if b, err := FromEnvValue("  "); b != nil || err != nil {
		t.Fatalf("leer: %v %v", b, err)
	}
	if _, err := FromEnvValue("kurz"); err == nil {
		t.Fatal("ungueltiger Schluessel angenommen")
	}
	b, err := FromEnvValue(base64.StdEncoding.EncodeToString(testKey(1)))
	if err != nil || b == nil {
		t.Fatalf("gueltig: %v", err)
	}
	// Ohne Schluessel weder ver- noch entschluesseln.
	var none *Box
	if _, err := none.Seal([]byte("x"), nil); !errors.Is(err, ErrNoKey) {
		t.Fatalf("Seal ohne Schluessel: %v", err)
	}
	if _, err := none.Open([]byte("x"), nil); !errors.Is(err, ErrNoKey) {
		t.Fatalf("Open ohne Schluessel: %v", err)
	}
}

func TestSealOpen(t *testing.T) {
	b, _ := New(testKey(1))
	plain := []byte("test-passwort-äö")
	aad := []byte("mandant-a")
	sealed, err := b.Seal(plain, aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if sealed[0] != 1 || len(sealed) != 1+12+len(plain)+16 {
		t.Fatalf("Format: Version %d, Laenge %d", sealed[0], len(sealed))
	}
	if bytes.Contains(sealed, plain) || strings.Contains(string(sealed), "passwort") {
		t.Fatal("Klartext im Chiffrat")
	}
	got, err := b.Open(sealed, aad)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("Open: %q %v", got, err)
	}
	// Leerer Klartext geht ebenfalls.
	if s, _ := b.Seal(nil, aad); s == nil {
		t.Fatal("leer")
	} else if got, err := b.Open(s, aad); err != nil || len(got) != 0 {
		t.Fatalf("leer oeffnen: %q %v", got, err)
	}
	// Jede Verschluesselung eine neue Nonce.
	again, _ := b.Seal(plain, aad)
	if bytes.Equal(again[1:13], sealed[1:13]) || bytes.Equal(again, sealed) {
		t.Fatal("Nonce wiederverwendet")
	}

	other, _ := New(testKey(2))
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	wrongVersion := append([]byte(nil), sealed...)
	wrongVersion[0] = 2
	for name, tc := range map[string]struct {
		box    *Box
		sealed []byte
		aad    []byte
	}{
		"anderer Schluessel": {other, sealed, aad},
		"andere AAD":         {b, sealed, []byte("mandant-b")},
		"ohne AAD":           {b, sealed, nil},
		"veraendert":         {b, tampered, aad},
		"andere Version":     {b, wrongVersion, aad},
		"zu kurz":            {b, sealed[:28], aad},
		"leer":               {b, nil, aad},
	} {
		if got, err := tc.box.Open(tc.sealed, tc.aad); !errors.Is(err, ErrOpen) || got != nil {
			t.Fatalf("%s: %q %v", name, got, err)
		}
	}
}
