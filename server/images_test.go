package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saintpete/logrole/services"
)

const imagepath = "/media.twiliocdn.com/AC58f1e8f2b1c6b88ca90a012a4be0c279/10a8a62e659081b0ac370192c3b9fb6b"

func TestGetImages(t *testing.T) {
	t.Parallel()
	ctype := "text/plain; from-test-server"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != imagepath {
			t.Errorf("expected URL.Path to equal %s, got %s", imagepath, r.URL.Path)
		}
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	key := services.NewRandomKey()
	u := services.Opaque(s.URL+imagepath, key)
	i := &imageServer{
		secretKey: key,
	}
	req, _ := http.NewRequest("GET", "/images/"+u, nil)
	w := httptest.NewRecorder()
	i.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected Code to be 200, got %d", w.Code)
	}
	if w.Body.String() != "hello world" {
		t.Errorf("expected 'hello world' body, got %s", w.Body.String())
	}
	if w.Header().Get("Content-Type") != ctype {
		t.Errorf("expected Content-Type to be %s, got %s", ctype, w.Header().Get("Content-Type"))
	}
}
