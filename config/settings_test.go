package config

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"testing"
)

func TestGetSecretKey(t *testing.T) {
	t.Parallel()
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

func TestNewSettingsFromEmptyConfig(t *testing.T) {
	t.Parallel()
	c := &FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
	}
	settings, err := NewSettingsFromConfig(c)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Client.AccountSid != "AC123" {
		t.Errorf("expected AccountSid to be AC123, got %s", settings.Client.AccountSid)
	}
	if settings.PageSize == 0 {
		t.Errorf("expected PageSize to be nonzero, got %d", settings.PageSize)
	}
	if settings.SecretKey == nil {
		t.Errorf("expected SecretKey to be non-nil, got %v", settings.SecretKey)
	}
}

func TestPolicyAndFileErrors(t *testing.T) {
	t.Parallel()
	c := &FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
		Policy:     new(Policy),
		PolicyFile: "/path/to/policy.yml",
	}
	_, err := NewSettingsFromConfig(c)
	if err == nil {
		t.Fatal("expected NewSettingsFromConfig to error, got nil")
	}
	if err.Error() != "Cannot define both policy and a policy_file" {
		t.Errorf("wrong error: %v", err)
	}
}

func TestInvalidPolicyRejected(t *testing.T) {
	t.Parallel()
	c := &FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
		Policy: &Policy{
			&Group{Name: ""},
		},
	}
	_, err := NewSettingsFromConfig(c)
	if err == nil {
		t.Fatal("expected NewSettingsFromConfig to error, got nil")
	}
	if err.Error() != "Group has no name, define a group name" {
		t.Errorf("wrong error: %v", err)
	}
}

func TestBasicAuthNoPolicyOK(t *testing.T) {
	t.Parallel()
	c := &FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
		User:       "test",
		Password:   "thepassword",
		AuthScheme: "basic",
		Policy:     nil,
	}
	if _, err := NewSettingsFromConfig(c); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyLoadedFromFile(t *testing.T) {
	t.Parallel()
	f, err := ioutil.TempFile("", "logrole-tests-")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	defer func(name string) {
		os.Remove(name)
	}(name)
	if err := ioutil.WriteFile(f.Name(), policy, 0644); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	c := &FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
		User:       "test",
		Password:   "thepassword",
		AuthScheme: "basic",
		PolicyFile: name,
	}
	if _, err := NewSettingsFromConfig(c); err != nil {
		t.Fatal(err)
	}
}
