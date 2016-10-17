package views

import (
	"testing"
	"time"

	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
)

var permissionsUser = config.NewUser(config.AllUserSettings())
var noPermissionsUser = config.NewUser(&config.UserSettings{})
var date = twilio.NewTwilioTime("Tue, 20 Sep 2016 22:59:57 +0000")

func TestCall(t *testing.T) {
	mv, err := NewMessage(&twilio.Message{
		Sid:         "SM123",
		DateCreated: *date,
	}, config.NewPermission(time.Hour+time.Now().Sub(date.Time)), permissionsUser)
	if err != nil {
		t.Fatal(err)
	}
	rv := NewRedactedMessage(mv)
	val, err := rv.Call("Sid")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := val.(string)
	if !ok {
		t.Errorf("tried to cast %v to string, not ok", val)
	}
	if s != "SM123" {
		t.Errorf("bad sid, want SM123, got %s", s)
	}
}

func TestCallUnknownReturnsError(t *testing.T) {
	mv, err := NewMessage(&twilio.Message{
		Sid:         "SM123",
		DateCreated: *date,
	}, config.NewPermission(time.Hour+time.Now().Sub(date.Time)), permissionsUser)
	if err != nil {
		t.Fatal(err)
	}
	rv := NewRedactedMessage(mv)
	_, err = rv.Call("Unknown")
	if err == nil {
		t.Errorf(`Call("Unknown"); expected error, got nil`)
	}
	if err.Error() != "Invalid method: Unknown" {
		t.Errorf("Got incorrect error: %v", err)
	}
}

func TestCallNoPermissionReturnsRedacted(t *testing.T) {
	mv, err := NewMessage(&twilio.Message{
		Sid:         "SM123",
		DateCreated: *date,
	}, config.NewPermission(time.Hour+time.Now().Sub(date.Time)), noPermissionsUser)
	if err != nil {
		t.Fatal(err)
	}
	rv := NewRedactedMessage(mv)
	sid, err := rv.Call("Sid")
	if err != nil {
		t.Fatal(err)
	}
	if sid != "[redacted]" {
		t.Errorf("Expected sid to be redacted, got %v", sid)
	}
}
