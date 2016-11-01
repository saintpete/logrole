package config

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saintpete/logrole/services"
)

func TestLoggedInAuthenticates(t *testing.T) {
	t.Parallel()
	key := services.NewRandomKey()
	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a := NewGoogleAuthenticator("", "", "http://localhost", nil, key)
	cookie := a.newCookie("user@example.com")
	req.AddCookie(cookie)
	_, err := a.Authenticate(w, req)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnknownUserWithValidDomainAllowed(t *testing.T) {
	// Should be allowed with the default user.
	t.Parallel()
	key := services.NewRandomKey()
	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a := NewGoogleAuthenticator("", "", "http://localhost", []string{"example.com"}, key)
	a.SetPolicy(&Policy{})
	cookie := a.newCookie("user@example.com")
	req.AddCookie(cookie)
	u, err := a.Authenticate(w, req)
	if err != nil {
		t.Fatal(err)
	}
	if u != DefaultUser {
		t.Errorf("expected to get DefaultUser, got %v", u)
	}
}

var authTests = []struct {
	policy      *Policy
	domains     []string
	id          string
	err         string
	defaultUser bool
}{
	// No domain, no policy
	{nil, nil, "user@example.com", "", true},
	// Not in policy, but known domain
	{nil, []string{"example.com"}, "user@example.com", "", true},
	// Check that we iterate through domains
	{nil, []string{"1.com", "2.com", "example.com", "3.com"}, "user@example.com", "", true},

	// User with an unknown domain, not in policy, shouldn't be allowed
	{nil, []string{"example.com"}, "user@unknown.example.com", "Need to login", false},
	// Policy has a default, no domains specified, use that
	{&Policy{&Group{Name: "1", Default: true}}, nil, "1@example.com", "", false},

	// Policy has a default but user's domain is not allowed, forbid them
	{&Policy{&Group{Name: "1", Default: true}}, []string{"example.com"}, "1@notallowed.example.com", "Need to login", false},

	// User not in domains, no default, but ID exists in policy
	{&Policy{&Group{Name: "1", Default: false, Users: []string{"1@allowed.example.com"}}}, []string{"example.com"}, "1@allowed.example.com", "", false},
}

func TestGoogleAuth(t *testing.T) {
	t.Parallel()
	key := services.NewRandomKey()
	for _, tt := range authTests {
		req, _ := http.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		a := NewGoogleAuthenticator("", "", "http://localhost", tt.domains, key)
		a.SetPolicy(tt.policy)
		cookie := a.newCookie(tt.id)
		req.AddCookie(cookie)
		user, err := a.Authenticate(w, req)
		if tt.err == "" && err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if tt.err != "" {
			if err == nil {
				t.Errorf("id: %s, policy: %v, domains: %v, expected non-nil error, got nil", tt.id, tt.policy, tt.domains)
			} else {
				if err.Error() != tt.err {
					t.Errorf("id: %s, policy: %v, domains: %v, expected err %s, got %v", tt.id, tt.policy, tt.domains, tt.err, err)
				}
			}
		}
		if tt.defaultUser && user != DefaultUser {
			t.Errorf("expected to get DefaultUser, didn't")
		}
	}
}
