package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saintpete/logrole/services"
)

//func TestUnknownUsersDenied(t *testing.T) {
//t.Parallel()
//settings := &Settings{
//AllowUnencryptedTraffic: true, Users: map[string]string{"test": "test"},
//SecretKey: services.NewRandomKey(),
//}
//s := NewServer(settings)
//req, _ := http.NewRequest("GET", "http://localhost:12345/foo", nil)
//req.SetBasicAuth("test", "wrongpassword")
//w := httptest.NewRecorder()
//s.ServeHTTP(w, req)
//if w.Code != 403 {
//t.Errorf("expected Code to be 403, got %d", w.Code)
//}
//}

func TestRequestsUpgraded(t *testing.T) {
	t.Parallel()
	settings := &Settings{AllowUnencryptedTraffic: false, SecretKey: services.NewRandomKey()}
	s := NewServer(settings)
	req, _ := http.NewRequest("GET", "http://localhost:12345/foo", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 301 {
		t.Errorf("expected Code to be 301, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	expected := "https://localhost:12345/foo"
	if location != expected {
		t.Errorf("expected Location header to be %s, got %s", expected, location)
	}
}

func TestIndex(t *testing.T) {
	t.Parallel()
	settings := &Settings{
		AllowUnencryptedTraffic: true,
		Authenticator:           &NoopAuthenticator{},
		SecretKey:               services.NewRandomKey(),
	}
	s := NewServer(settings)
	req, _ := http.NewRequest("GET", "http://localhost:12345/", nil)
	req.SetBasicAuth("test", "test")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "server error") {
		t.Errorf("Got unexpected server error, body: %s", w.Body.String())
	}
}

func TestStaticPagesAvailableNoAuth(t *testing.T) {
	t.Parallel()
	settings := &Settings{
		SecretKey:     services.NewRandomKey(),
		Authenticator: NewBasicAuthAuthenticator("logrole", map[string]string{"test": "test"}),
	}
	s := NewServer(settings)
	req, _ := http.NewRequest("GET", "http://localhost:12345/static/css/style.css", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
}

//type data struct{}

//func (d *data) Foo() (string, error) {
//return "", errors.New("bad")
//}

//func TestRender(t *testing.T) {
//tpl := template.Must(template.New("t").Option("missingkey=error").Funcs(funcMap).Parse(`
//{{ (call redacted .Foo) }}
//`))
//d := &data{}
//b := new(bytes.Buffer)
//err := tpl.Execute(b, d)
//if err != nil {
//t.Fatal(err)
//}
//fmt.Println(b.String())
//t.Fail()
//}
