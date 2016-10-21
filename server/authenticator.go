package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var loginTemplate *template.Template

func init() {
	base := string(assets.MustAsset("templates/base.html"))
	templates := template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base))

	tindex := template.Must(templates.Clone())
	loginTpl := string(assets.MustAsset("templates/login.html"))
	loginTemplate = template.Must(tindex.Parse(loginTpl))
}

type Authenticator interface {
	// Authenticate ensures the request is authenticated. If it is not
	// authenticated, or authentication returns an error, Authenticate will
	// write a response and return a non-nil error.
	Authenticate(w http.ResponseWriter, r *http.Request) (*config.User, error)
}

type NoopAuthenticator struct{}

func (n *NoopAuthenticator) Authenticate(w http.ResponseWriter, r *http.Request) (*config.User, error) {
	// TODO
	return theUser, nil
}

// AddAuthenticator adds the Authenticator as a HTTP middleware. If
// authentication is successful, we set the User in the request context and
// continue.
func AddAuthenticator(h http.Handler, a Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := a.Authenticate(w, r)
		if err != nil {
			return
		}
		r = config.SetUser(r, u)
		h.ServeHTTP(w, r)
	})
}

type BasicAuthAuthenticator struct {
	Realm string
	Users map[string]string
}

func NewBasicAuthAuthenticator(realm string, users map[string]string) *BasicAuthAuthenticator {
	return &BasicAuthAuthenticator{Realm: realm, Users: users}
}

func (b *BasicAuthAuthenticator) Authenticate(w http.ResponseWriter, r *http.Request) (*config.User, error) {
	// Implementation mostly taken from handlers/lib.go:BasicAuth. Would be
	// nice to figure out how a way to reuse that code instead of copying it.
	user, pass, ok := r.BasicAuth()
	if !ok {
		rest.Unauthorized(w, r, b.Realm)
		return nil, &rest.Error{Title: "No Basic Auth"}
	}
	serverPass, ok := b.Users[user]
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
	return LookupUser(user)
}

func LookupUser(name string) (*config.User, error) {
	// TODO user lookup
	return theUser, nil
}

type GoogleAuthenticator struct {
	AllowUnencryptedTraffic bool
	Conf                    *oauth2.Config
	secretKey               *[32]byte
}

func NewGoogleAuthenticator(clientID string, clientSecret string, baseURL string, secretKey *[32]byte) *GoogleAuthenticator {
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
		Conf:      conf,
		secretKey: secretKey,
	}
}

type loginData struct {
	Start    time.Time
	Duration time.Duration
	Path     string
	URL      string
}

type state struct {
	CurrentURL string
	Time       time.Time
}

func (l *loginData) Title() string {
	return "Log In"
}

func (g *GoogleAuthenticator) renderLoginPage(w http.ResponseWriter, r *http.Request) {
	st := state{
		CurrentURL: r.URL.RequestURI(),
		Time:       time.Now().UTC(),
	}
	bits, err := json.Marshal(st)
	if err != nil {
		rest.ServerError(w, r, err)
		return
	}
	encoded := services.OpaqueByte(bits, g.secretKey)
	data := &loginData{
		Start: time.Now(),
		Path:  r.URL.Path,
		URL:   g.Conf.AuthCodeURL(encoded),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(401)
	if err := render(w, loginTemplate, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
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
		rest.ServerError(w, r, err)
		return err
	}

	client := g.Conf.Client(ctx, tok)
	u, err := services.GetGoogleUserData(client)
	if err != nil {
		rest.ServerError(w, r, err)
		return err
	}
	cookie := g.newCookie(u.Email)
	http.SetCookie(w, cookie)
	http.Redirect(w, r, currentURL, 302)
	return errors.New("redirected, make another request")
}

func (g *GoogleAuthenticator) Authenticate(w http.ResponseWriter, r *http.Request) (*config.User, error) {
	if r.URL.Path == "/auth/callback" {
		err := g.handleGoogleCallback(w, r)
		return nil, err
	}
	// Check if the request has a valid cookie, if so allow it.
	cookie, err := r.Cookie("token")
	if err != nil {
		// render the login page.
		g.renderLoginPage(w, r)
		return nil, err
	}
	fmt.Printf("cookie: %#v\n", cookie)
	val, err := services.UnopaqueByte(cookie.Value, g.secretKey)
	if err != nil {
		// need a 400 bad request here
		g.renderLoginPage(w, r)
		return nil, err
	}
	t := new(token)
	if err := json.Unmarshal(val, t); err != nil {
		g.renderLoginPage(w, r)
		return nil, err
	}
	if t.Expiry.Before(time.Now().UTC()) {
		// TODO logout
		g.renderLoginPage(w, r)
		return nil, err
	}
	// TODO return different users
	return LookupUser(t.ID)
}

// TODO add different users, or pull from database
//var theUser = config.NewUser(config.AllUserSettings())

var theUser = config.NewUser(&config.UserSettings{
	CanViewNumMedia:       true,
	CanViewMessages:       true,
	CanViewMessageFrom:    true,
	CanViewMessageTo:      true,
	CanViewMessageBody:    true,
	CanViewMessagePrice:   false,
	CanViewMedia:          true,
	CanViewCalls:          true,
	CanViewCallFrom:       true,
	CanViewCallTo:         true,
	CanViewCallPrice:      false,
	CanViewNumRecordings:  true,
	CanPlayRecordings:     true,
	CanViewRecordingPrice: false,
})
