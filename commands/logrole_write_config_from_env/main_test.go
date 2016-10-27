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

func TestWriteConfig(t *testing.T) {
	e := &dummyEnvironment{
		env: map[string]string{"PORT": "56789"},
	}
	buf := new(bytes.Buffer)
	writeConfig(buf, e)
	expected := "port: 56789\n\n"
	if s := buf.String(); s != expected {
		t.Errorf("expected config to be %s, got %s", expected, s)
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
