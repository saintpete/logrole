package server

import (
	"net/http"
	"net/http/httptest"
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
