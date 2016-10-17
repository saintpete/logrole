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
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
)

const Version = "0.7"

var year = time.Now().UTC().Year()

var funcMap = template.FuncMap{
	"year":          func() int { return year },
	"friendly_date": services.FriendlyDate,
	"duration":      services.Duration,
	"render":        render,
}

func render(start time.Time) string {
	since := time.Since(start)
	return services.Duration(since)
}

func init() {
	staticServer = &static{
		modTime: time.Now().UTC(),
	}

	base := string(assets.MustAsset("templates/base.html"))
	templates := template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base))

	tindex := template.Must(templates.Clone())
	indexTpl := string(assets.MustAsset("templates/index.html"))
	indexTemplate = template.Must(tindex.Parse(indexTpl))
}

type static struct {
	modTime time.Time
}

func AuthUserHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r, _, err := config.AuthUser(r)
		if err != nil {
			rest.Forbidden(w, r, &rest.Error{
				Title: err.Error(),
			})
			return
		}
		h.ServeHTTP(w, r)
	})
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

type indexServer struct{}

var indexTemplate *template.Template

func (i *indexServer) Title() string {
	return "Logrole Homepage"
}

func (i *indexServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.ExecuteTemplate(w, "base", nil); err != nil {
		rest.ServerError(w, r, err)
	}
}

type Settings struct {
	// The host the user visits to get to this site.
	PublicHost              string
	AllowUnencryptedTraffic bool
	Users                   map[string]string
	Client                  *twilio.Client

	// A string like "America/New_York". Defaults to UTC if not set.
	Location *time.Location

	// How many messages to display per page.
	MessagesPageSize uint

	// Used to encrypt next page URI's and sessions. See config.sample.yml for
	// more information.
	SecretKey *[32]byte

	MaxResourceAge time.Duration
}

// TODO different users or pull from database
var theUser = config.NewUser(&config.UserSettings{
	CanViewMessages:    true,
	CanViewNumMedia:    true,
	CanViewMessageFrom: true,
	CanViewMessageTo:   true,
	CanViewMedia:       true,
	CanViewMessageBody: true,
})

// NewServer returns a new Handler that can serve requests. If the users map is
// empty, Basic Authentication is disabled.
func NewServer(settings *Settings) http.Handler {
	validKey := false
	for i := 0; i < len(settings.SecretKey); i++ {
		if settings.SecretKey[i] != 0x0 {
			validKey = true
			break
		}
	}
	if !validKey {
		panic("server: nil secret key in settings")
	}

	// TODO persistent storage
	for name := range settings.Users {
		config.AddUser(name, theUser)
	}

	permission := config.NewPermission(settings.MaxResourceAge)
	vc := views.NewClient(handlers.Logger, settings.Client, settings.SecretKey, permission)
	mls := &messageListServer{
		Client:    settings.Client,
		Location:  settings.Location,
		PageSize:  settings.MessagesPageSize,
		SecretKey: settings.SecretKey,
	}
	mis := &messageInstanceServer{
		Client:   vc,
		Location: settings.Location,
	}
	if settings.Location == nil {
		mls.Location = time.UTC
		mis.Location = time.UTC
	}
	ss := &searchServer{}
	o := &openSearchXMLServer{
		PublicHost:              settings.PublicHost,
		AllowUnencryptedTraffic: settings.AllowUnencryptedTraffic,
	}
	index := &indexServer{}
	image := &imageServer{
		SecretKey: settings.SecretKey,
	}
	r := new(handlers.Regexp)
	r.Handle(regexp.MustCompile(`^/$`), []string{"GET"}, index)
	r.Handle(imageRoute, []string{"GET"}, image)
	r.Handle(regexp.MustCompile(`^/search$`), []string{"GET"}, ss)
	r.Handle(regexp.MustCompile(`^/opensearch.xml$`), []string{"GET"}, o)
	r.Handle(regexp.MustCompile(`^/messages$`), []string{"GET"}, mls)
	r.Handle(messageInstanceRoute, []string{"GET"}, mis)
	r.Handle(regexp.MustCompile(`^/static`), []string{"GET"}, staticServer)
	var h http.Handler = UpgradeInsecureHandler(r, settings.AllowUnencryptedTraffic)
	if len(settings.Users) > 0 {
		// TODO database, remove duplication
		h = AuthUserHandler(h)
		h = handlers.BasicAuth(h, "logrole", settings.Users)
	}
	return handlers.Duration(
		handlers.Log(
			handlers.Debug(
				handlers.TrailingSlashRedirect(
					handlers.UUID(
						handlers.Server(h, "logrole/"+Version),
					),
				),
			),
		),
	)
}
