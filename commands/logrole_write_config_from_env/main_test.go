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
