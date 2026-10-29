// Package secretbox verschluesselt Geheimnisse, die in der Datenbank liegen
// muessen (ADR 0033, Backlog F.14.1) - etwa das Passwort des Mail-Zugangs.
//
// AES-256-GCM; der Schluessel kommt aus SETTINGS_ENCRYPTION_KEY und liegt
// nie in der Datenbank. Format: Version (1 Byte) || Nonce (12) || Chiffrat
// mit Tag. Die AAD bindet ein Chiffrat an seinen Ort (z. B. die
// Mandanten-ID): an anderer Stelle laesst es sich nicht entschluesseln.
//
// Fehlermeldungen nennen weder Schluessel noch Inhalt.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

const (
	version1  = 1
	keyLength = 32
)

var (
	// ErrNoKey: es ist kein Schluessel eingerichtet.
	ErrNoKey = errors.New("Kein Schlüssel für verschlüsselte Einstellungen eingerichtet (SETTINGS_ENCRYPTION_KEY)")
	// ErrOpen: das Chiffrat ist beschaedigt, gehoert an einen anderen Ort
	// oder wurde mit einem anderen Schluessel erzeugt.
	ErrOpen = errors.New("Gespeichertes Geheimnis lässt sich nicht entschlüsseln")
)

// Box ver- und entschluesselt mit einem festen Schluessel.
type Box struct {
	aead cipher.AEAD
}

// ParseKey liest einen Base64-kodierten Schluessel von 32 Byte.
func ParseKey(s string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, errors.New("SETTINGS_ENCRYPTION_KEY ist kein gültiges Base64")
	}
	if len(key) != keyLength {
		return nil, errors.New("SETTINGS_ENCRYPTION_KEY muss 32 Byte lang sein (z. B. 'openssl rand -base64 32')")
	}
	return key, nil
}

// New erzeugt eine Box aus einem Schluessel von 32 Byte.
func New(key []byte) (*Box, error) {
	if len(key) != keyLength {
		return nil, errors.New("Schlüssel muss 32 Byte lang sein")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// FromEnvValue erzeugt die Box aus dem Wert der Umgebungsvariablen. Leer
// ergibt (nil, nil): kein Schluessel eingerichtet - Seal und Open einer
// nil-Box liefern dann ErrNoKey.
func FromEnvValue(s string) (*Box, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	key, err := ParseKey(s)
	if err != nil {
		return nil, err
	}
	return New(key)
}

// Seal verschluesselt plain, gebunden an aad. Jeder Aufruf nimmt eine
// neue Nonce.
func (b *Box) Seal(plain, aad []byte) ([]byte, error) {
	if b == nil {
		return nil, ErrNoKey
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 1+len(nonce)+len(plain)+b.aead.Overhead())
	out = append(out, version1)
	out = append(out, nonce...)
	return b.aead.Seal(out, nonce, plain, aad), nil
}

// Open entschluesselt ein Chiffrat aus Seal mit derselben aad.
func (b *Box) Open(sealed, aad []byte) ([]byte, error) {
	if b == nil {
		return nil, ErrNoKey
	}
	n := b.aead.NonceSize()
	if len(sealed) < 1+n+b.aead.Overhead() || sealed[0] != version1 {
		return nil, ErrOpen
	}
	plain, err := b.aead.Open(nil, sealed[1:1+n], sealed[1+n:], aad)
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}
