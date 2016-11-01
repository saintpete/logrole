package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/server"
)

func startServer(t *testing.T, c *config.FileConfig) *server.Server {
	settings, err := config.NewSettingsFromConfig(c)
	if err != nil {
		t.Fatal(err)
	}
	s, err := server.NewServer(settings)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestServerStartsWithOnlySidAndToken(t *testing.T) {
	t.Parallel()
	c := &config.FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
	}
	s := startServer(t, c)
	defer s.Close()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
}

func TestBasicAuthOnlyAllowsKnownUser(t *testing.T) {
	t.Parallel()
	c := &config.FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
		AuthScheme: "basic",
		User:       "test",
		Password:   "thepassword",
	}
	s := startServer(t, c)
	defer s.Close()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	s.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected Code to be 401, got %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/", nil)
	req2.SetBasicAuth("test", "thepassword")
	s.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w2.Code)
	}
}

func TestBasicAuthNoUserPolicyRejected(t *testing.T) {
	t.Parallel()
	c := &config.FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
		User:       "test",
		Password:   "thepassword",
		AuthScheme: "basic",
		Policy: &config.Policy{
			&config.Group{Name: "test"},
		},
	}
	s := startServer(t, c)
	defer s.Close()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	s.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected Code to be 401, got %d", w.Code)
	}
}
