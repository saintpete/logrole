package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saintpete/logrole/services"
)

func TestLoginRedirect(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a := NewGoogleAuthenticator("", "", "http://localhost", services.NewRandomKey())
	a.Authenticate(w, req)
	if w.Code != 302 {
		t.Errorf("expected Code to be 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login?g=/" {
		t.Errorf("expected redirect to /login?g=/, got %s", loc)
	}
}

func TestGoogleAuthenticatorRendersLoginPage(t *testing.T) {
	t.Parallel()
	a := NewGoogleAuthenticator("", "", "http://localhost", services.NewRandomKey())
	req, _ := http.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	a.Authenticate(w, req)
	if w.Code != 401 {
		t.Errorf("expected Code to be 401, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Log in with Google") {
		t.Errorf("expected Body to contain 'Log in with Google', got %s", body)
	}
}

func TestLoggedInAuthenticates(t *testing.T) {
	key := services.NewRandomKey()
	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a := NewGoogleAuthenticator("", "", "http://localhost", key)
	cookie := a.newCookie("user@example.com")
	req.AddCookie(cookie)
	_, err := a.Authenticate(w, req)
	if err != nil {
		t.Fatal(err)
	}
}
