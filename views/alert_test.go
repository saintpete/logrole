package views

import (
	"testing"
	"time"

	twilio "github.com/saintpete/twilio-go"
	"github.com/saintpete/logrole/config"
)

func TestViewResourceSid(t *testing.T) {
	permission := config.NewPermission(2 * time.Hour)
	s := config.AllUserSettings()
	s.CanViewCalls = false
	s.CanViewMessages = true
	talert := &twilio.Alert{Sid: "NO123", ResourceSid: "CA123", DateCreated: twilio.TwilioTime{Valid: true, Time: time.Now()}}
	alert, err := NewAlert(talert, config.NewPermission(1000*1000*time.Hour), config.NewUser(s))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alert.ResourceSid(); err != config.PermissionDenied {
		t.Fatalf("expected to get PermissionDenied, got %v", err)
	}
	talert.ResourceSid = "MM123"
	alert, err = NewAlert(talert, permission, config.NewUser(s))
	if err != nil {
		t.Fatal(err)
	}
	sid, err := alert.ResourceSid()
	if err != nil {
		t.Fatal(err)
	}
	if sid != "MM123" {
		t.Errorf("wrong Sid")
	}
}
