package services

import (
	"testing"
	"time"
)

func TestFriendlyLocation(t *testing.T) {
	l, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if fl := FriendlyLocation(l); fl != "New York" {
		t.Errorf("FriendlyLocation('America/New_York') should equal 'New York', got %s", fl)
	}
}
