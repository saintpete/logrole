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
func Opaque(s string, secretKey *[32]byte) (string, error) {
	nonce := NewNonce()
	encrypted := secretbox.Seal(nonce[:], []byte(s), nonce, secretKey)
	return base64.URLEncoding.EncodeToString(encrypted), nil
}

var errTooShort = errors.New("services: Encrypted string is too short")
var errInvalidInput = errors.New("services: Could not decrypt invalid input")

// Unopaque decodes compressed using base64, then decrypts the decoded byte
// array using the secretKey.
func Unopaque(compressed string, secretKey *[32]byte) (string, error) {
	encrypted, err := base64.URLEncoding.DecodeString(compressed)
	if err != nil {
		return "", err
	}
	if len(encrypted) < 24 {
		return "", errTooShort
	}
	decryptNonce := new([24]byte)
	copy(decryptNonce[:], encrypted[:24])
	decrypted, ok := secretbox.Open([]byte{}, encrypted[24:], decryptNonce, secretKey)
	if !ok {
		return "", errInvalidInput
	}
	return string(decrypted), nil
}

// Duration returns a friendly duration (with the insignificant bits rounded
// off).
func Duration(d time.Duration) string {
	d2 := (d / (100 * time.Microsecond)) * (100 * time.Microsecond)
	return d2.String()
}
