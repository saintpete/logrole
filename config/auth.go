package config

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	"github.com/saintpete/logrole/services"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Authenticator interface {
	SetPolicy(*Policy)
	// Authenticate ensures the request is authenticated. If it is not
	// authenticated, or authentication returns an error, Authenticate will
	// write a response and return a non-nil error.
	Authenticate(http.ResponseWriter, *http.Request) (*User, error)
	Logout(http.ResponseWriter, *http.Request)
}

// NoopAuthenticator returns the given User in response to all Authenticate
// requests.
type NoopAuthenticator struct {
	User *User
}

func (n *NoopAuthenticator) Authenticate(w http.ResponseWriter, r *http.Request) (*User, error) {
	if n.User != nil {
		return n.User, nil
	} else {
		return DefaultUser, nil
	}
}

func (n *NoopAuthenticator) Logout(w http.ResponseWriter, r *http.Request) {}

// SetPolicy does nothing.
func (n *NoopAuthenticator) SetPolicy(p *Policy) {}

// BasicAuthAuthenticator can authenticate users via Basic Auth. Call
// AddUserPassword to set a Basic Auth user/password combo, and SetPolicy to
// set the Policy for authenticated users. If no Policy has been set,
// DefaultUser will be returned for all authenticated users.
type BasicAuthAuthenticator struct {
	Realm string
	// Passwords holds a map of usernames/passwords for basic auth. The keys
	// should match the keys in the Users map.
	Passwords map[string]string
	Policy    *Policy
	mu        sync.Mutex
}

func NewBasicAuthAuthenticator(realm string) *BasicAuthAuthenticator {
	return &BasicAuthAuthenticator{
		Realm:     realm,
		Passwords: make(map[string]string),
	}
}

// SetPolicy sets the policy. Call AddUserPassword to set a Basic Auth user /
// password.
func (b *BasicAuthAuthenticator) SetPolicy(p *Policy) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Policy = p
}

// AddUserPassword sets a user and password for Basic Auth. AddUserPassword
// overrides any previous passwords that have been set for key. Call
// AddUserPassword with an empty password to remove a user.
func (b *BasicAuthAuthenticator) AddUserPassword(key string, password string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if password == "" {
		delete(b.Passwords, key)
		return
	}
	b.Passwords[key] = password
}

// Authenticate checks whether the request was made with a valid user/password
// via Basic Auth. When authenticating, if the Basic Auth user is in the
// policy, that user's permissions are used. If no user is available, but a
// policy is defined and it contains a "default" group, those permissions
// are used. If no policy is present, config.DefaultUser is returned for
// authenticated users.
func (b *BasicAuthAuthenticator) Authenticate(w http.ResponseWriter, r *http.Request) (*User, error) {
	// Implementation mostly taken from handlers/lib.go:BasicAuth. Would be
	// nice to figure out how a way to reuse that code instead of copying it.
	user, pass, ok := r.BasicAuth()
	if !ok {
		rest.Unauthorized(w, r, b.Realm)
		return nil, &rest.Error{Title: "No Basic Auth"}
	}
	serverPass, ok := b.Passwords[user]
	if !ok {
		var err *rest.Error
		if user == "" {
			rest.Unauthorized(w, r, b.Realm)
			err = &rest.Error{Title: "No credentials"}
		} else {
			err = &rest.Error{
				Title: "Username or password are invalid. Please double check your credentials",
				ID:    "forbidden",
			}
			rest.Forbidden(w, r, err)
		}
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(pass), []byte(serverPass)) != 1 {
		err := &rest.Error{
			Title:    fmt.Sprintf("Incorrect password for user %s", user),
			ID:       "incorrect_password",
			Instance: r.URL.Path,
		}
		rest.Forbidden(w, r, err)
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Policy == nil {
		return DefaultUser, nil
	} else {
		u, _, err := b.Policy.Lookup(user)
		if err != nil {
			rest.Unauthorized(w, r, b.Realm)
			return nil, &rest.Error{Title: "User not found"}
		}
		return u, nil
	}
}

func (b *BasicAuthAuthenticator) Logout(w http.ResponseWriter, r *http.Request) {
	// There's apparently no good way to do this.
	// http://stackoverflow.com/a/449914/329700
}

type GoogleAuthenticator struct {
	AllowUnencryptedTraffic bool
	Conf                    *oauth2.Config
	RenderLogin             func(http.ResponseWriter, *http.Request, string)
	RenderLogout            func(http.ResponseWriter, *http.Request)
	allowedDomains          []string
	secretKey               *[32]byte
	policy                  *Policy
	mu                      sync.Mutex
}

func NewGoogleAuthenticator(clientID string, clientSecret string, baseURL string, allowedDomains []string, secretKey *[32]byte) *GoogleAuthenticator {
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  baseURL + "/auth/callback",
		// https://developers.google.com/identity/protocols/googlescopes#google_sign-in
		Scopes: []string{
			"profile",
			"email",
		},
		Endpoint: google.Endpoint,
	}
	return &GoogleAuthenticator{
		Conf:           conf,
		allowedDomains: allowedDomains,
		secretKey:      secretKey,
	}
}

type state struct {
	CurrentURL string
	Time       time.Time
}

type OAuthAuthenticator interface {
	URL(http.ResponseWriter, *http.Request) string
}

func (g *GoogleAuthenticator) URL(w http.ResponseWriter, r *http.Request) string {
	var uri string
	if g := r.URL.Query().Get("g"); g != "" {
		// prevent open redirect by only using the Path part
		u, err := url.Parse(g)
		if err == nil {
			uri = u.Path
		} else {
			uri = r.URL.RequestURI()
		}
	} else {
		uri = r.URL.RequestURI()
	}
	st := state{
		CurrentURL: uri,
		Time:       time.Now().UTC(),
	}
	bits, err := json.Marshal(st)
	if err != nil {
		rest.ServerError(w, r, err)
		return ""
	}
	encoded := services.OpaqueByte(bits, g.secretKey)
	return g.Conf.AuthCodeURL(encoded)
}

const AuthTimeout = 1 * time.Hour

func (g *GoogleAuthenticator) validState(encrypted string) (string, bool) {
	b, err := services.UnopaqueByte(encrypted, g.secretKey)
	if err != nil {
		return "", false
	}
	st := new(state)
	if err := json.Unmarshal(b, st); err != nil {
		return "", false
	}
	if time.Since(st.Time) > AuthTimeout {
		return "", false
	}
	return st.CurrentURL, true
}

const GoogleTimeout = 5 * time.Second

type token struct {
	ID     string
	Expiry time.Time
}

func newToken(id string) *token {
	return &token{
		ID:     id,
		Expiry: time.Now().UTC().Add(14 * 24 * time.Hour),
	}
}

func (g *GoogleAuthenticator) newCookie(id string) *http.Cookie {
	t := newToken(id)
	b, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	text := services.OpaqueByte(b, g.secretKey)
	return &http.Cookie{
		Name:     "token",
		Value:    text,
		Path:     "/",
		Secure:   g.AllowUnencryptedTraffic == false,
		Expires:  t.Expiry,
		HttpOnly: true,
	}
}

func (g *GoogleAuthenticator) handleGoogleCallback(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()
	st := query.Get("state")
	currentURL, ok := g.validState(st)
	if !ok {
		http.Redirect(w, r, "/", 302)
		return errors.New("invalid state")
	}
	code := query.Get("code")
	if code == "" {
		handlers.Logger.Warn("Callback request has valid state, no code")
		http.Redirect(w, r, "/", 302)
		return errors.New("invalid state")
	}
	ctx, cancel := context.WithTimeout(r.Context(), GoogleTimeout)
	defer cancel()
	tok, err := g.Conf.Exchange(ctx, code)
	if err != nil {
		// TODO this can return 400+JSON if you try to redeem a code twice:
		// Response: {
		//  "error" : "invalid_grant",
		//  "error_description" : "Invalid code."
		// }
		rest.ServerError(w, r, err)
		return err
	}

	client := g.Conf.Client(ctx, tok)
	u, err := services.GetGoogleUserData(ctx, client)
	if err != nil {
		rest.ServerError(w, r, err)
		return err
	}
	_, lookupErr := g.lookupUser(u.Email)
	if lookupErr != nil {
		restErr := &rest.Error{
			Title: lookupErr.Error(),
			ID:    "unauthorized_domain",
		}
		// TODO - better error message here and also don't show the Logout
		// link since you are not logged in
		rest.Forbidden(w, r, restErr)
		return lookupErr
	}
	cookie := g.newCookie(u.Email)
	http.SetCookie(w, cookie)
	http.Redirect(w, r, currentURL, 302)
	return errors.New("redirected, make another request")
}

func (g *GoogleAuthenticator) permitted(id string) error {
	if len(g.allowedDomains) > 0 {
		domainMatch := false
		for _, domain := range g.allowedDomains {
			if strings.HasSuffix(id, "@"+domain) {
				domainMatch = true
				break
			}
		}
		if !domainMatch {
			return errors.New("Email " + id + " is from a domain that is not authorized to access this site")
		}
	}
	return nil
}

var MustLogin = errors.New("Need to login")

func (g *GoogleAuthenticator) Authenticate(w http.ResponseWriter, r *http.Request) (*User, error) {
	if r.URL.Path == "/auth/callback" {
		err := g.handleGoogleCallback(w, r)
		return nil, err
	}
	// Check if the request has a valid cookie, if so allow it.
	cookie, err := r.Cookie("token")
	if err != nil {
		return nil, MustLogin
	}
	val, err := services.UnopaqueByte(cookie.Value, g.secretKey)
	if err != nil {
		// need a 400 bad request here
		return nil, MustLogin
	}
	t := new(token)
	if err := json.Unmarshal(val, t); err != nil {
		return nil, MustLogin
	}
	if t.Expiry.Before(time.Now().UTC()) {
		// TODO logout
		return nil, MustLogin
	}
	// if you got to this point you have a valid login cookie, don't show you
	// the login page.
	if r.URL.Path == "/login" {
		http.Redirect(w, r, "/", 302)
		return nil, errors.New("redirected logged in user to homepage")
	}
	u, err := g.lookupUser(t.ID)
	if err != nil {
		if err == MustLogin {
			g.Logout(w, r)
		}
		return nil, err
	}
	return u, nil
}

func (g *GoogleAuthenticator) lookupUser(id string) (*User, error) {
	if g.policy == nil {
		// no policy, only check whether domain is permitted and return
		// DefaultUser
		if err := g.permitted(id); err == nil {
			return DefaultUser, nil
		} else {
			handlers.Logger.Warn("User has valid login but does not have a permitted domain", "id", id)
			return nil, MustLogin
		}
	}

	u, ok, err := g.policy.Lookup(id)
	if ok {
		return u, nil
	}
	permittedErr := g.permitted(id)
	switch {
	case permittedErr != nil:
		// User domain not allowed.
		handlers.Logger.Warn("User not found by ID in policy, domain is not allowed", "id", id)
		return nil, MustLogin
	case err == nil:
		// We found a default user in the policy, and they're permitted
		return u, nil
	case err != nil:
		// No default user, but this user has a valid domain
		return DefaultUser, nil
	default:
		panic("unreachable")
	}
}

func (g *GoogleAuthenticator) SetPolicy(p *Policy) {
	g.mu.Lock()
	g.policy = p
	g.mu.Unlock()
}

func (g *GoogleAuthenticator) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Secure:   g.AllowUnencryptedTraffic == false,
		HttpOnly: true,
		MaxAge:   -1,
		Path:     "/",
	})
	http.Redirect(w, r, "/", 302)
}
