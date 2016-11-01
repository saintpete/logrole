package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"
)

var googleResponse = []byte(`{
 "sub": "112158670131180366716",
 "name": "Kevin Burke",
 "given_name": "Kevin",
 "family_name": "Burke",
 "profile": "https://plus.google.com/112158670131180366716",
 "picture": "https://lh3.googleusercontent.com/-SeM4jx8yves/AAAAAAAAAAI/AAAAAAAAAA8/jVfDI8kQlaE/photo.jpg",
 "email": "kev@inburke.com",
 "email_verified": true,
 "gender": "male",
 "locale": "en",
 "hd": "inburke.com"
} `)

func TestGoogle(t *testing.T) {
	t.Parallel()
	oldBaseURL := UserDataBase
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(googleResponse)
	}))
	UserDataBase = s.URL
	defer func() {
		s.Close()
		UserDataBase = oldBaseURL
	}()
	u, err := GetGoogleUserData(context.Background(), http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "kev@inburke.com" {
		t.Errorf("email: got %s, want kev@inburke.com", u.Email)
	}
}
