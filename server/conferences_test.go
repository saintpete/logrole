package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/test"
	"github.com/saintpete/logrole/test/harness"
)

func TestUnauthorizedUserCantViewConferenceList(t *testing.T) {
	t.Parallel()
	vc := harness.ViewsClient(harness.ViewHarness{})
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
	vc := harness.ViewsClient(harness.ViewHarness{})
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

func TestGetConferenceFiltersGeneratesCorrectQuery(t *testing.T) {
	t.Parallel()
	expected := "/2010-04-01/Accounts/AC123/Conferences.json?DateCreated%3C=2016-10-28&DateCreated%3E=2016-10-27&PageSize=1"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != expected {
			t.Errorf("expected URL to be %s, got %s", expected, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(test.CallListBody) // todo better fake
	}))
	defer s.Close()
	vc := harness.ViewsClient(harness.ViewHarness{SecretKey: key, TestServer: s})
	c, err := newConferenceListServer(dlog, vc, lf, 1, config.DefaultMaxResourceAge, key)
	if err != nil {
		t.Fatal(err)
	}
	// 22:34 NYC time gets converted to 2:34 next day UTC
	req, _ := http.NewRequest("GET", "/conferences?created-before=2016-10-27T19:25&created-after=2016-10-26T22:34", nil)
	req.SetBasicAuth("test", "test")
	req = config.SetUser(req, theUser)
	w := httptest.NewRecorder()
	c.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
}
