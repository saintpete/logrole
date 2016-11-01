package main

import (
	"bytes"
	"testing"
)

type dummyEnvironment struct {
	env map[string]string
}

func (d *dummyEnvironment) LookupEnv(key string) (string, bool) {
	val, ok := d.env[key]
	return val, ok
}

var expectedConfig = `port: 56789
public_host: localhost:7

twilio_account_sid: AC123

timezones:
  - America/Los_Angeles
  - America/New_York

`

func TestWriteConfig(t *testing.T) {
	t.Parallel()
	e := &dummyEnvironment{
		env: map[string]string{
			"PORT":               "56789",
			"PUBLIC_HOST":        "localhost:7",
			"TWILIO_ACCOUNT_SID": "AC123",
			"TIMEZONES":          "America/Los_Angeles,America/New_York",
		},
	}
	buf := new(bytes.Buffer)
	writeConfig(buf, e)
	if s := buf.String(); s != expectedConfig {
		t.Errorf("expected config to be %s, got %s", expectedConfig, s)
	}
}

func TestWriteCommaConfig(t *testing.T) {
	t.Parallel()
	e := &dummyEnvironment{
		env: map[string]string{"PORT": "56789", "GOOGLE_ALLOWED_DOMAINS": "example.net,example.com,example.org"},
	}
	buf := new(bytes.Buffer)
	writeConfig(buf, e)
	expected := `port: 56789

google_allowed_domains:
  - example.net
  - example.com
  - example.org

`
	if s := buf.String(); s != expected {
		t.Errorf("expected config to be %s, got %s", expected, s)
	}
}

func TestValidatePolicy(t *testing.T) {
	t.Parallel()
	e := &dummyEnvironment{
		env: map[string]string{"POLICY_FILE": "foo", "POLICY_URL": "blah"},
	}
	err := validatePolicy(e)
	if err == nil {
		t.Fatal("expected err to be non-nil, got nil")
	}
	if err.Error() != "Cannot specify both POLICY_FILE and POLICY_URL" {
		t.Errorf("expected 'Cannot specify' error, got %v", err)
	}
}

func TestValidateSecureURL(t *testing.T) {
	t.Parallel()
	e := &dummyEnvironment{
		env: map[string]string{"POLICY_URL": "http://google.com"},
	}
	err := validatePolicy(e)
	if err == nil {
		t.Fatal("expected err to be non-nil, got nil")
	}
	if err.Error() != "Cowardly refusing to download policy file (http://google.com) over insecure scheme. Use HTTPS" {
		t.Errorf("Wrong error: %v", err)
	}
}

func TestValidateUnknownURL(t *testing.T) {
	t.Parallel()
	e := &dummyEnvironment{
		env: map[string]string{"POLICY_URL": "weird"},
	}
	err := validatePolicy(e)
	if err == nil {
		t.Fatal("expected err to be non-nil, got nil")
	}
	if err.Error() != "I don't know how to download a file from weird" {
		t.Errorf("Wrong error: %v", err)
	}
}
