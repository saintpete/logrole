// Package services implements utility functions.
package services

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"time"

	"golang.org/x/crypto/nacl/secretbox"
)

// NewRandomKey returns a random key or panics if one cannot be provided.
func NewRandomKey() *[32]byte {
	key := new([32]byte)
	if _, err := io.ReadFull(rand.Reader, key[:]); err != nil {
		panic(err)
	}
	return key
}

func NewNonce() *[24]byte {
	nonce := new([24]byte)
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		panic(err)
	}
	return nonce
}

// Opaque encrypts s with secretKey and returns the encrypted string encoded
// with base64, or an error.
func Opaque(s string, secretKey *[32]byte) string {
	return OpaqueByte([]byte(s), secretKey)
}

func OpaqueByte(b []byte, secretKey *[32]byte) string {
	nonce := NewNonce()
	encrypted := secretbox.Seal(nonce[:], b, nonce, secretKey)
	return base64.URLEncoding.EncodeToString(encrypted)
}

var errTooShort = errors.New("services: Encrypted string is too short")
var errInvalidInput = errors.New("services: Could not decrypt invalid input")

// Unopaque decodes compressed using base64, then decrypts the decoded byte
// array using the secretKey.
func Unopaque(compressed string, secretKey *[32]byte) (string, error) {
	b, err := UnopaqueByte(compressed, secretKey)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func UnopaqueByte(compressed string, secretKey *[32]byte) ([]byte, error) {
	encrypted, err := base64.URLEncoding.DecodeString(compressed)
	if err != nil {
		return nil, err
	}
	if len(encrypted) < 24 {
		return nil, errTooShort
	}
	decryptNonce := new([24]byte)
	copy(decryptNonce[:], encrypted[:24])
	decrypted, ok := secretbox.Open([]byte{}, encrypted[24:], decryptNonce, secretKey)
	if !ok {
		return nil, errInvalidInput
	}
	return decrypted, nil
}

// Duration returns a friendly duration (with the insignificant bits rounded
// off).
func Duration(d time.Duration) string {
	if d > 10*time.Second {
		d2 := (d / (100 * time.Millisecond)) * (100 * time.Millisecond)
		return d2.String()
	}
	if d > time.Second {
		d2 := (d / (10 * time.Millisecond)) * (10 * time.Millisecond)
		return d2.String()
	}
	d2 := (d / (100 * time.Microsecond)) * (100 * time.Microsecond)
	return d2.String()
}

// TruncateSid truncates the Sid to the first 6 characters of the ID (16
// million possibilities).
func TruncateSid(sid string) string {
	if len(sid) > 8 {
		return sid[:8]
	}
	return sid
}

// FriendlyDate returns a friendlier version of the date.
func FriendlyDate(t time.Time) string {
	return friendlyDate(t, time.Now().UTC())
}

func friendlyDate(t time.Time, utcnow time.Time) string {
	now := utcnow.In(t.Location())
	y, m, d := now.Date()
	if d == t.Day() && m == t.Month() && y == t.Year() {
		return t.Format("3:04pm")
	}
	y1, m1, d1 := now.Add(-24 * time.Hour).Date()
	if d1 == t.Day() && m1 == t.Month() && y1 == t.Year() {
		return t.Format("Yesterday, 3:04pm")
	}
	// if the same year, return the day
	if y == t.Year() {
		return t.Format("3:04pm, January 2")
	}
	return t.Format("3:04pm, January 2, 2006")
}
