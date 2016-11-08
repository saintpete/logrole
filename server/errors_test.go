package server

import (
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strings"
	"testing"

	"github.com/kevinburke/rest"
)

func clearErrorHandlers() {
	rest.RegisterHandler(400, nil)
	rest.RegisterHandler(401, nil)
	rest.RegisterHandler(403, nil)
	rest.RegisterHandler(404, nil)
	rest.RegisterHandler(405, nil)
	rest.RegisterHandler(500, nil)
}

func TestErrorsRender(t *testing.T) {
	t.Parallel()
	defer clearErrorHandlers()
	es, _ := newErrorServer(nil, nil)
	registerErrorHandlers(es)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	rest.NotFound(w, req)
	if w.Code != 404 {
		t.Errorf("expected Code to be 404, got %d", w.Code)
	}
	if ctype := w.Header().Get("Content-Type"); ctype != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type to be text/html, got %s", ctype)
	}
	if body := w.Body.String(); !strings.Contains(body, "<h2>Page Not Found</h2>") {
		t.Errorf("expected body to contain Not Found, got %s", body)
	}
}

func Test401RendersHTML(t *testing.T) {
	t.Parallel()
	defer clearErrorHandlers()
	es, _ := newErrorServer(nil, nil)
	registerErrorHandlers(es)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	rest.Unauthorized(w, req, "domain")
	if ctype := w.Header().Get("Content-Type"); ctype != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type to be text/html, got %s", ctype)
	}
}

func TestErrorShowsEmail(t *testing.T) {
	t.Parallel()
	address, _ := mail.ParseAddress("test@example.com")
	defer clearErrorHandlers()
	es, _ := newErrorServer(address, nil)
	registerErrorHandlers(es)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	rest.NotFound(w, req)
	if body := w.Body.String(); !strings.Contains(body, "test@example.com") {
		t.Errorf("expected body to contain test@example.com, got %s", body)
	}
}
