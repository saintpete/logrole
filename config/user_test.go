package config

import (
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"
)

func TestUnmarshal(t *testing.T) {
	yml := []byte(`
can_view_num_media: true
can_view_messages: false
`)
	us := new(UserSettings)
	if err := yaml.Unmarshal(yml, us); err != nil {
		t.Fatal(err)
	}
	if us.CanViewNumMedia == false {
		t.Errorf("expected CanViewNumMedia to be true, got false")
	}
	if us.CanViewMessages == true {
		t.Errorf("expected CanViewMessages to be false, got true")
	}
	// unspecified should default to true
	if us.CanViewMessageFrom == false {
		t.Errorf("expected CanViewMessageFrom to be true, got false")
	}
}

func TestCanViewResource(t *testing.T) {
	u := &User{maxResourceAge: 0}
	now := time.Now()
	if u.CanViewResource(now, 0) == false {
		t.Errorf("with both values zero, CanViewResource should be true, got false")
	}
	if u.CanViewResource(now, time.Hour) == false {
		t.Errorf("with global Age == time.Hour, CanViewResource should be true, got false")
	}
	if u.CanViewResource(now, time.Nanosecond) == true {
		t.Errorf("with global Age == time.Nanosecond, CanViewResource should be false, got true")
	}
	u.maxResourceAge = time.Minute
	if u.CanViewResource(now, time.Nanosecond) == false {
		t.Errorf("with local Age = time.Minute, global Age == time.Nanosecond, CanViewResource should be true, got false")
	}
	if u.CanViewResource(now.Add(-2*time.Minute), time.Hour) == true {
		t.Errorf("with local Age = time.Minute, global Age == time.Nanosecond, CanViewResource (2 minutes ago) should be false, got true")
	}
}
