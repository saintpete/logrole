package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/server"
)

func startBenchServer(b *testing.B, c *config.FileConfig) *server.Server {
	settings, err := config.NewSettingsFromConfig(c, NullLogger)
	if err != nil {
		b.Fatal(err)
	}
	s, err := server.NewServer(settings)
	if err != nil {
		b.Fatal(err)
	}
	return s
}

func BenchmarkRenderLoginPage(b *testing.B) {
	c := &config.FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
		AuthScheme: "basic",
		User:       "test",
		Password:   "password",
	}
	s := startBenchServer(b, c)
	defer s.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		req.SetBasicAuth("test", "password")
		s.ServeHTTP(w, req)
		if w.Code != 200 {
			b.Fatalf("non-200 error code %d", w.Code)
		}
		b.SetBytes(int64(w.Body.Len()))
	}
}

func BenchmarkRenderMessageList(b *testing.B) {
	c := &config.FileConfig{
		AccountSid: "AC123",
		AuthToken:  "123",
		AuthScheme: "basic",
		User:       "test",
		Password:   "password",
	}
	// note this will only get hit once or twice, after that we store/serve the
	// response from the cache
	twilioServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(MessageBody)
	}))
	defer twilioServer.Close()
	settings, err := config.NewSettingsFromConfig(c, NullLogger)
	if err != nil {
		b.Fatal(err)
	}
	settings.Client.Base = twilioServer.URL
	s, err := server.NewServer(settings)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/messages", nil)
		req.SetBasicAuth("test", "password")
		s.ServeHTTP(w, req)
		if w.Code != 200 {
			b.Fatalf("non-200 error code %d", w.Code)
		}
		b.SetBytes(int64(w.Body.Len()))
	}
}
