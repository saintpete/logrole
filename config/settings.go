package config

import (
	"net/http"
	"net/mail"
	"time"

	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/services"
)

const DefaultPort = "4114"
const DefaultPageSize = 50

var DefaultTimezones = []string{
	"America/Los_Angeles",
	"America/Denver",
	"America/Chicago",
	"America/New_York",
}

// DefaultMaxResourceAge allows all resources to be fetched. The company was
// founded in 2008, so there should definitely be no resources created in the
// 1980's.
var DefaultMaxResourceAge = time.Since(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))

// Settings are used to configure a Server and apply to all of the website's
// users.
type Settings struct {
	// The host the user visits to get to this site.
	PublicHost              string
	AllowUnencryptedTraffic bool
	Client                  *twilio.Client

	LocationFinder services.LocationFinder

	// How many messages to display per page.
	PageSize uint

	// Used to encrypt next page URI's and sessions. See config.sample.yml for
	// more information.
	SecretKey *[32]byte

	// Don't show resources that are older than this age.
	MaxResourceAge time.Duration

	// Should a user have to click a button to view media attached to a MMS?
	ShowMediaByDefault bool

	// Email address for server errors / "contact me" on error pages.
	Mailto *mail.Address

	// Error reporter. This must not be nil; set to NoopErrorReporter to ignore
	// errors.
	Reporter services.ErrorReporter

	// The authentication scheme.
	Authenticator Authenticator
}

type Authenticator interface {
	// Authenticate ensures the request is authenticated. If it is not
	// authenticated, or authentication returns an error, Authenticate will
	// write a response and return a non-nil error.
	Authenticate(w http.ResponseWriter, r *http.Request) (*User, error)
	Logout(w http.ResponseWriter, r *http.Request)
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
		r = SetUser(r, u)
		h.ServeHTTP(w, r)
	})
}
