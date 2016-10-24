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
)

var dlog = log.New()
var key = services.NewRandomKey()

func init() {
	dlog.SetHandler(log.DiscardHandler())
}

func TestInvalidNext(t *testing.T) {
	s := &messageListServer{
		Logger:    dlog,
		SecretKey: key,
	}
	config.AddUser("test", theUser)
	enc := services.Opaque("invalid", key)
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

func newServerWithResponse(code int, resp []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		w.Write(resp)
	}))
}

func Test404OnResource404(t *testing.T) {
	server := newServerWithResponse(404, notFoundResp)
	defer server.Close()
	harness := test.ViewHarness{TestServer: server}
	vc := test.ViewsClient(harness)
	s := &messageInstanceServer{Logger: dlog, Client: vc}
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

var oldResults = []byte(`{"first_page_uri": "/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages.json?From=%2B19252717005&PageSize=1&Page=0", "end": 1, "previous_page_uri": "/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages.json?From=%2B19252717005&PageSize=1&Page=0&PageToken=PBSMcc61f9140a65752eadf1351d6ccd0f15", 
"messages": [
{"sid": "SMcc61f9140a65752eadf1351d6ccd0f15", "date_created": "Tue, 20 Sep 2016 22:41:38 +0000", "date_updated": "Tue, 20 Sep 2016 22:41:39 +0000", "date_sent": "Tue, 20 Sep 2016 22:41:39 +0000", "account_sid": "AC58f1e8f2b1c6b88ca90a012a4be0c279", "to": "+19253920364", "from": "+19252717005", "messaging_service_sid": null, "body": "Hello", "status": "received", "num_segments": "1", "num_media": "0", "direction": "inbound", "api_version": "2010-04-01", "price": "-0.00750", "price_unit": "USD", "error_code": null, "error_message": null, "uri": "/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages/SMcc61f9140a65752eadf1351d6ccd0f15.json", "subresource_uris": 
{"media": "/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages/SMcc61f9140a65752eadf1351d6ccd0f15/Media.json"}
}], 
"uri": "/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages.json?From=%2B19252717005&PageSize=1&Page=1&PageToken=PAMM89a8c4a6891c53054e9cd604922bfb61", "page_size": 1, "start": 1, "next_page_uri": "/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages.json?From=%2B19252717005&PageSize=1&Page=2&PageToken=PASMcc61f9140a65752eadf1351d6ccd0f15", "page": 1}`)

var uris = []string{
	"/messages",
	"/messages?next=" + services.Opaque("/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages.json?PageSize=50&Page=1&PageToken=PASM0ea5868a88542cc21fd0f85c4daa6c33", key),
}

func TestNoResultsIfAllResultsOld(t *testing.T) {
	t.Parallel()
	server := newServerWithResponse(200, oldResults)
	defer server.Close()
	// date_created above
	tt := twilio.NewTwilioTime("Tue, 20 Sep 2016 22:41:38 +0000")
	// max resource age is 1 hour newer than the message age
	age := time.Since(tt.Time) - time.Hour
	harness := test.ViewHarness{TestServer: server, SecretKey: key, MaxResourceAge: age}
	vc := test.ViewsClient(harness)
	s := &messageListServer{Logger: dlog, SecretKey: key, Client: vc}
	// TODO this is all very clunky
	config.AddUser("test", theUser)
	for _, uri := range uris {
		req, _ := http.NewRequest("GET", uri, nil)
		req.SetBasicAuth("test", "test")
		req, _, _ = config.AuthUser(req)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if w.Code != 200 {
			fmt.Printf("%#v\n", w.Header())
			fmt.Println(w.Body.String())
			t.Errorf("expected Code to be 200, got %d", w.Code)
		}
	}
}
