package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
)

func TestInvalidNext(t *testing.T) {
	key := services.NewRandomKey()
	s := &messageListServer{
		SecretKey: key,
	}
	config.AddUser("test", theUser)
	enc, _ := services.Opaque("invalid", key)
	req, _ := http.NewRequest("GET", "/messages?next="+enc, nil)
	req.SetBasicAuth("test", "test")
	req, _, _ = config.AuthUser(req)
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

func Test404OnResource404(t *testing.T) {
	twilioServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(404)
		w.Write(notFoundResp)
	}))
	defer twilioServer.Close()
	c := twilio.NewClient("AC123", "123", nil)
	c.Base = twilioServer.URL
	vc := views.NewClient(nil, c, nil, nil)
	s := &messageInstanceServer{Client: vc}
	// TODO this is all very clunky
	config.AddUser("test", theUser)
	req, _ := http.NewRequest("GET", "/messages/MMd04242a0544234abba080942e0535505", nil)
	req.SetBasicAuth("test", "test")
	req, _, _ = config.AuthUser(req)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected Code to be 404, got %d", w.Code)
	}
}
