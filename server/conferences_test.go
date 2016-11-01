package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/test"
)

func TestUnauthorizedUserCantViewConferenceList(t *testing.T) {
	t.Parallel()
	vc := test.ViewsClient(test.ViewHarness{})
	s, err := newConferenceListServer(dlog, vc, nil, 50, time.Hour, key)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "/conferences", nil)
	req.SetBasicAuth("test", "test")
	u := config.NewUser(&config.UserSettings{CanViewConferences: false})
	req = config.SetUser(req, u)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("expected to get 403, got %d", w.Code)
	}
}

func TestUnauthorizedUserCantViewConferenceInstance(t *testing.T) {
	t.Parallel()
	vc := test.ViewsClient(test.ViewHarness{})
	s, err := newConferenceInstanceServer(dlog, vc, nil)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "/conferences/CF6c38e4202f499c5020dd3ca679010779", nil)
	req.SetBasicAuth("test", "test")
	u := config.NewUser(&config.UserSettings{CanViewConferences: false})
	req = config.SetUser(req, u)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("expected to get 403, got %d", w.Code)
	}
}
