package server

import (
	"bytes"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/services"
)

const Version = "0.3"

var year = time.Now().UTC().Year()

var funcMap = template.FuncMap{
	"year":          func() int { return year },
	"friendly_date": services.FriendlyDate,
	"shorturl":      services.Shorter,
	"duration":      services.Duration,
}

func init() {
	staticServer = &static{
		modTime: time.Now().UTC(),
	}
}

type static struct {
	modTime time.Time
}

func UpgradeInsecureHandler(h http.Handler, allowUnencryptedTraffic bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowUnencryptedTraffic == false {
			if r.Header.Get("X-Forwarded-Proto") == "http" {
				u := r.URL
				u.Scheme = "https"
				u.Host = r.Host
				http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
				return
			}
		}
		// This header doesn't mean anything when served over HTTP, but
		// detecting HTTPS is a general way is hard, so let's just send it
		// every time.
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		h.ServeHTTP(w, r)
	})
}

var staticServer http.Handler

func (s *static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bits, err := assets.Asset(strings.TrimPrefix(r.URL.Path, "/"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		rest.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, r.URL.Path, s.modTime, bytes.NewReader(bits))
}

type Settings struct {
	// The host the user visits to get to this site.
	PublicHost              string
	AllowUnencryptedTraffic bool
	Users                   map[string]string
	Client                  *twilio.Client
	Location                *time.Location
}

// NewServer returns a new Handler that can serve requests. If the users map is
// empty, Basic Authentication is disabled.
func NewServer(settings *Settings) http.Handler {
	mls := &messageListServer{
		Client:   settings.Client,
		Location: settings.Location,
	}
	mis := &messageInstanceServer{
		Client:   settings.Client,
		Location: settings.Location,
	}
	if settings.Location == nil {
		mls.Location = time.UTC
		mis.Location = time.UTC
	}
	o := &openSearchXMLServer{
		PublicHost:              settings.PublicHost,
		AllowUnencryptedTraffic: settings.AllowUnencryptedTraffic,
	}
	r := new(handlers.Regexp)
	r.Handle(regexp.MustCompile(`^/opensearch.xml$`), []string{"GET"}, o)
	r.Handle(regexp.MustCompile(`^/messages$`), []string{"GET"}, mls)
	r.Handle(messageInstanceRoute, []string{"GET"}, mis)
	r.Handle(regexp.MustCompile(`^/static`), []string{"GET"}, staticServer)
	var h http.Handler = r
	if len(settings.Users) > 0 {
		h = handlers.BasicAuth(r, "logrole", settings.Users)
	}
	return handlers.Duration(
		handlers.Log(
			handlers.Debug(
				handlers.TrailingSlashRedirect(
					handlers.UUID(
						handlers.Server(
							UpgradeInsecureHandler(h, settings.AllowUnencryptedTraffic),
							"logrole/"+Version),
					),
				),
			),
		),
	)
}
