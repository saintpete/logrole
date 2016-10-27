package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/views"
)

func TestUnauthorizedUserCantViewConferenceList(t *testing.T) {
	vc := views.NewClient(dlog, nil, nil, nil)
	s, err := newConferenceListServer(dlog, vc, nil, 50, time.Hour, key)
	if err != nil {
		t.Fatal(err)
	}
	u := config.NewUser(&config.UserSettings{CanViewConferences: false})
	config.AddUser("test", u)
	req, _ := http.NewRequest("GET", "/conferences", nil)
	req.SetBasicAuth("test", "test")
	req, _, _ = config.AuthUser(req)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("expected to get 403, got %d", w.Code)
	}
}
