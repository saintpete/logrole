package main

import (
	"encoding/hex"
	"testing"
)

func TestGetSecretKey(t *testing.T) {
	key, err := getSecretKey("")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if key[0] == 0x0 && key[1] == 0x0 && key[2] == 0x0 && key[3] == 0x0 {
		t.Error("expected key to be filled with random data, got 0's")
	}

	if _, err := getSecretKey("wrong length"); err != errWrongLength {
		t.Errorf("expected wrong-length error, got %v", err)
	}

	_, err = getSecretKey("zzzzzz6e676520746869732070617373776f726420746f206120736563726574")
	if err == nil || err.Error() != "encoding/hex: invalid byte: U+007A 'z'" {
		t.Errorf("expected invalid hex error, got %v", err)
	}

	key, err = getSecretKey("6368616e676520746869732070617373776f726420746f206120736563726574")
	if err != nil {
		t.Fatal(err)
	}
	h := hex.EncodeToString(key[:])
	if h != "6368616e676520746869732070617373776f726420746f206120736563726574" {
		t.Errorf("could not roundtrip decoded key: %s", h)
	}
}
