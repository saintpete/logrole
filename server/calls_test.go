package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/test"
	"github.com/saintpete/logrole/test/harness"
)

func TestGetFiltersGeneratesCorrectQuery(t *testing.T) {
	t.Parallel()
	expected := "/Accounts/AC123/Calls.json?PageSize=1&StartTime%3C=2016-10-28&StartTime%3E=2016-10-27"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != expected {
			t.Errorf("expected URL to be %s, got %s", expected, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(test.CallListBody)
	}))
	defer s.Close()
	vc := harness.ViewsClient(harness.ViewHarness{SecretKey: key, TestServer: s})
	c, err := newCallListServer(dlog, vc, lf, 1, config.DefaultMaxResourceAge, key)
	if err != nil {
		t.Fatal(err)
	}
	// 22:34 NYC time gets converted to 2:34 next day UTC
	req, _ := http.NewRequest("GET", "/calls?start-before=2016-10-27T19:25&start-after=2016-10-26T22:34", nil)
	req.SetBasicAuth("test", "test")
	req = config.SetUser(req, theUser)
	w := httptest.NewRecorder()
	c.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
}

func TestNoEndGeneratesCorrectQuery(t *testing.T) {
	t.Parallel()
	expected := "/Accounts/AC123/Calls.json?PageSize=1&StartTime%3E=2016-10-27"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != expected {
			t.Errorf("expected URL to be %s, got %s", expected, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(test.CallListBody)
	}))
	defer s.Close()
	vc := harness.ViewsClient(harness.ViewHarness{SecretKey: key, TestServer: s})
	c, err := newCallListServer(dlog, vc, lf, 1, config.DefaultMaxResourceAge, key)
	if err != nil {
		t.Fatal(err)
	}
	// 22:34 NYC time gets converted to 2:34 next day UTC
	req, _ := http.NewRequest("GET", "/calls?start-after=2016-10-26T22:34", nil)
	req.SetBasicAuth("test", "test")
	req = config.SetUser(req, theUser)
	w := httptest.NewRecorder()
	c.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
}
