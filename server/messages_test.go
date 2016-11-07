package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	log "github.com/inconshreveable/log15"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/test"
	"github.com/saintpete/logrole/test/harness"
)

var dlog = log.New()
var key = services.NewRandomKey()
var lf services.LocationFinder

func init() {
	dlog.SetHandler(log.DiscardHandler())
	var err error
	lf, err = services.NewLocationFinder("America/New_York")
	if err != nil {
		panic(err)
	}
}

func TestInvalidNext(t *testing.T) {
	t.Parallel()
	vc := harness.ViewsClient(harness.ViewHarness{SecretKey: key})
	s, err := newMessageListServer(dlog, vc, lf, 50, time.Hour, key)
	if err != nil {
		t.Fatal(err)
	}
	enc := services.Opaque("invalid", key)
	req, _ := http.NewRequest("GET", "/messages?next="+enc, nil)
	req.SetBasicAuth("test", "test")
	req = config.SetUser(req, theUser)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected Code to be 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid next page uri") {
		t.Errorf("expected Body to contain error message, got %s", w.Body.String())
	}
}

// invalid status here on purpose to check we use a different one.
var notFoundResp = []byte("{\"code\": 20404, \"message\": \"The requested resource /2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Calls/unknown.json was not found\", \"more_info\": \"https://www.twilio.com/docs/errors/20404\", \"status\": 428}")

func newServerWithResponse(code int, resp []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		w.Write(resp)
	}))
}

func Test404OnResource404(t *testing.T) {
	t.Parallel()
	server := newServerWithResponse(404, notFoundResp)
	defer server.Close()
	hrns := harness.ViewHarness{TestServer: server}
	vc := harness.ViewsClient(hrns)
	s := &messageInstanceServer{Logger: dlog, Client: vc}
	req, _ := http.NewRequest("GET", "/messages/MMd04242a0544234abba080942e0535505", nil)
	req.SetBasicAuth("test", "test")
	req = config.SetUser(req, theUser)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected Code to be 404, got %d", w.Code)
	}
}

var uris = []string{
	"/messages",
	"/messages?next=" + services.Opaque("/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages.json?PageSize=50&Page=1&PageToken=PASM0ea5868a88542cc21fd0f85c4daa6c33", key),
}

func TestNoResultsIfAllResultsOld(t *testing.T) {
	t.Parallel()
	server := newServerWithResponse(200, test.OldMessageBody)
	defer server.Close()
	// date_created above
	tt := twilio.NewTwilioTime("Tue, 20 Sep 2016 22:41:38 +0000")
	// max resource age is 1 hour newer than the message age
	age := time.Since(tt.Time) - time.Hour
	hrns := harness.ViewHarness{TestServer: server, SecretKey: key, MaxResourceAge: age}
	vc := harness.ViewsClient(hrns)
	lf, _ := services.NewLocationFinder("America/Los_Angeles")
	s, err := newMessageListServer(dlog, vc, lf, 50, time.Hour, key)
	if err != nil {
		t.Fatal(err)
	}
	for _, uri := range uris {
		req, _ := http.NewRequest("GET", uri, nil)
		req.SetBasicAuth("test", "test")
		req = config.SetUser(req, theUser)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if w.Code != 200 {
			fmt.Printf("%#v\n", w.Header())
			//fmt.Println(w.Body.String())
			t.Errorf("expected Code to be 200, got %d", w.Code)
		}
	}
}
