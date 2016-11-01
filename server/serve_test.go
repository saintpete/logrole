package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
)

//func TestUnknownUsersDenied(t *testing.T) {
//t.Parallel()
//settings := &Settings{
//AllowUnencryptedTraffic: true, Users: map[string]string{"test": "test"},
//SecretKey: services.NewRandomKey(),
//}
//s := NewServer(settings)
//req, _ := http.NewRequest("GET", "http://localhost:12345/foo", nil)
//req.SetBasicAuth("test", "wrongpassword")
//w := httptest.NewRecorder()
//s.ServeHTTP(w, req)
//if w.Code != 403 {
//t.Errorf("expected Code to be 403, got %d", w.Code)
//}
//}

func TestRequestsUpgraded(t *testing.T) {
	t.Parallel()
	settings := &config.Settings{AllowUnencryptedTraffic: false, SecretKey: key}
	s, err := NewServer(settings)
	if err != nil {
		t.Fatal(err)
	}
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

func TestIndex(t *testing.T) {
	t.Parallel()
	settings := &config.Settings{
		AllowUnencryptedTraffic: true,
		Authenticator:           &config.NoopAuthenticator{},
		SecretKey:               services.NewRandomKey(),
	}
	s, err := NewServer(settings)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://localhost:12345/", nil)
	req.SetBasicAuth("test", "test")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "server error") {
		t.Errorf("Got unexpected server error, body: %s", w.Body.String())
	}
}

func getGoogleAuthServer(t *testing.T) *Server {
	key := services.NewRandomKey()
	settings := &config.Settings{
		SecretKey:     key,
		Authenticator: config.NewGoogleAuthenticator("", "", "http://localhost", nil, key),
	}
	s, err := NewServer(settings)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGoogleAuthenticatorRendersLoginPage(t *testing.T) {
	t.Parallel()
	s := getGoogleAuthServer(t)
	req, _ := http.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected Code to be 401, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Log in with Google") {
		t.Errorf("expected Body to contain 'Log in with Google', got %s", body)
	}
}

func TestGoogleAuthenticatorRedirects(t *testing.T) {
	t.Parallel()
	s := getGoogleAuthServer(t)
	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 302 {
		t.Errorf("expected Code to be 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login?g=/" {
		t.Errorf("expected redirect to /login?g=/, got %s", loc)
	}
}

func TestStaticPagesAvailableNoAuth(t *testing.T) {
	t.Parallel()
	a := config.NewBasicAuthAuthenticator("logrole")
	a.AddUserPassword("test", "test")
	settings := &config.Settings{
		SecretKey:     services.NewRandomKey(),
		Authenticator: a,
	}
	s, err := NewServer(settings)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://localhost:12345/static/css/style.css", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
}
