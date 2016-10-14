package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saintpete/logrole/services"
)

func TestUnknownUsersDenied(t *testing.T) {
	settings := &Settings{
		AllowUnencryptedTraffic: true, Users: map[string]string{"test": "test"},
		SecretKey: services.NewRandomKey(),
	}
	s := NewServer(settings)
	req, _ := http.NewRequest("GET", "http://localhost:12345/foo", nil)
	req.SetBasicAuth("test", "wrongpassword")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("expected Code to be 403, got %d", w.Code)
	}
}

func TestRequestsUpgraded(t *testing.T) {
	settings := &Settings{AllowUnencryptedTraffic: false, SecretKey: services.NewRandomKey()}
	s := NewServer(settings)
	req, _ := http.NewRequest("GET", "http://localhost:12345/foo", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 301 {
		t.Errorf("expected Code to be 301, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	expected := "https://localhost:12345/foo"
	if location != expected {
		t.Errorf("expected Location header to be %s, got %s", expected, location)
	}
}
