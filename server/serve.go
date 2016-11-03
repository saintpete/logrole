// Package server responds to incoming HTTP requests and renders the site.
//
// There are a number of smaller servers in this package, each of which takes
// only the configuration necessary to serve it.
package server

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
)

// Server version, run "make release" to increase this value
const Version = "0.61"

func getRemoteIP(r *http.Request) string {
	fwd := r.Header.Get("X-Forwarded-For")
	if fwd == "" {
		return r.RemoteAddr
	}
	return strings.Split(fwd, ",")[0]
}

// whitelistIPs checks whether the request's IP address was made from an IP
// inside the provided ranges of ips. WhitelistIPs uses the first value in the
// request's X-Forwarded-For header (if one is present), or r.RemoteAddr if an
// X-Forwarded-For header is not present.
//
// THIS IS NOT A SECURITY FEATURE. It is possible to spoof IP addresses or
// construct an X-Forwarded-For header that contains a different IP address
// than the request's originating address.
func whitelistIPs(h http.Handler, l log.Logger, nets []*net.IPNet) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ipStr := getRemoteIP(r)
		// RemoteHost reports both
		host, _, err := net.SplitHostPort(ipStr)
		if err == nil {
			ipStr = host
		}
		ip := net.ParseIP(ipStr)
		found := false
		if ip == nil {
			l.Warn("Could not parse X-Forwarded-For header or RemoteHost as IP address. Allowing access", "ip", ipStr)
			found = true
		} else {
			for _, n := range nets {
				if n.Contains(ip) {
					found = true
					break
				}
			}
		}
		if !found {
			l.Warn("Denying access to request based on IP", "ip", ipStr, "subnets", nets)
			rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
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

// Static file HTTP server; all assets are packaged up in the assets directory
// with go-bindata.
type static struct {
	modTime time.Time
}

func (s *static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		r.URL.Path = "/static/favicon.ico"
	}
	bits, err := assets.Asset(strings.TrimPrefix(r.URL.Path, "/"))
	if err != nil {
		rest.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, r.URL.Path, s.modTime, bytes.NewReader(bits))
}

type indexServer struct{}

type indexData struct {
	baseData
}

func (i *indexData) Title() string {
	return "Homepage"
}

func (i *indexServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := &baseData{Data: &indexData{}}
	if err := render(w, r, indexTemplate, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

type Server struct {
	http.Handler
	vc       views.Client
	DoneChan chan bool
	PageSize uint
}

func (s *Server) Close() error {
	s.DoneChan <- true
	return nil
}

func (s *Server) CacheCommonQueries() {
	go s.vc.CacheCommonQueries(s.PageSize, s.DoneChan)
}

type loginData struct {
	baseData
	URL string
}

func (l *loginData) Title() string {
	return "Log In"
}

func GoogleLoginRenderer() func(http.ResponseWriter, *http.Request, string) {
	return func(w http.ResponseWriter, r *http.Request, url string) {
	}
}

type loginServer struct{}

func (ls *loginServer) Serve(w http.ResponseWriter, r *http.Request, URL string) {
	if r.URL.Path != "/login" {
		http.Redirect(w, r, "/login?g="+r.URL.Path, 302)
		return
	}
	bd := &baseData{
		LoggedOut: true,
	}
	bd.Data = &loginData{
		URL: URL,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(401)
	if err := render(w, r, loginTemplate, "base", bd); err != nil {
		rest.ServerError(w, r, err)
	}
}

// AddAuthenticator adds the Authenticator as a HTTP middleware. If
// authentication is successful, we set the User in the request context and
// continue.
func AddAuthenticator(h http.Handler, ls *loginServer, a config.Authenticator) http.Handler {
	// TODO
	o, ok := a.(*config.GoogleAuthenticator)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := a.Authenticate(w, r)
		if err == config.MustLogin {
			var url string
			if ok {
				url = o.URL(w, r)
			}
			ls.Serve(w, r, url)
			return
		}
		if err != nil {
			return
		}
		r = config.SetUser(r, u)
		h.ServeHTTP(w, r)
	})
}

// NewServer returns a new Handler that can serve the website.
func NewServer(settings *config.Settings) (*Server, error) {
	if settings.Reporter == nil {
		settings.Reporter = services.GetReporter("noop", "")
	}
	validKey := false
	for i := 0; i < len(settings.SecretKey); i++ {
		if settings.SecretKey[i] != 0x0 {
			validKey = true
			break
		}
	}
	if !validKey {
		return nil, errors.New("Invalid secret key (must initialize some bytes)")
	}
	if settings.Authenticator == nil {
		settings.Authenticator = &config.NoopAuthenticator{}
	}
	if settings.Logger == nil {
		return nil, errors.New("Please configure a non-nil Logger")
	}
	permission := config.NewPermission(settings.MaxResourceAge)
	vc := views.NewClient(settings.Logger, settings.Client, settings.SecretKey, permission)
	mls, err := newMessageListServer(settings.Logger, vc, settings.LocationFinder,
		settings.PageSize, settings.MaxResourceAge, settings.SecretKey)
	if err != nil {
		return nil, err
	}
	mis, err := newMessageInstanceServer(settings.Logger, vc, settings.LocationFinder, settings.ShowMediaByDefault)
	if err != nil {
		return nil, err
	}
	cls, err := newCallListServer(settings.Logger, vc, settings.LocationFinder,
		settings.PageSize, settings.MaxResourceAge, settings.SecretKey)
	if err != nil {
		return nil, err
	}
	cis, err := newCallInstanceServer(settings.Logger, vc, settings.LocationFinder)
	if err != nil {
		return nil, err
	}
	confs, err := newConferenceListServer(settings.Logger, vc,
		settings.LocationFinder, settings.PageSize, settings.MaxResourceAge,
		settings.SecretKey)
	if err != nil {
		return nil, err
	}
	confInstance, err := newConferenceInstanceServer(settings.Logger, vc,
		settings.LocationFinder)
	if err != nil {
		return nil, err
	}
	als, err := newAlertListServer(settings.Logger, vc,
		settings.LocationFinder, settings.PageSize, settings.MaxResourceAge,
		settings.SecretKey)
	if err != nil {
		return nil, err
	}
	ss := &searchServer{}
	o := &openSearchXMLServer{
		PublicHost:              settings.PublicHost,
		AllowUnencryptedTraffic: settings.AllowUnencryptedTraffic,
	}
	index := &indexServer{}
	image := &imageServer{
		secretKey: settings.SecretKey,
	}
	proxy, err := newAudioReverseProxy()
	if err != nil {
		return nil, err
	}
	audio := &audioServer{
		Client:    vc,
		Proxy:     proxy,
		secretKey: settings.SecretKey,
	}
	staticServer := &static{
		modTime: time.Now().UTC(),
	}
	logout := &logoutServer{
		Authenticator: settings.Authenticator,
	}
	ls := &loginServer{}
	tz := &tzServer{
		Logger:                  settings.Logger,
		AllowUnencryptedTraffic: settings.AllowUnencryptedTraffic,
		LocationFinder:          settings.LocationFinder,
	}

	e := &errorServer{
		Mailto:   settings.Mailto,
		Reporter: settings.Reporter,
	}
	registerErrorHandlers(e)

	authR := new(handlers.Regexp)
	authR.Handle(regexp.MustCompile(`^/$`), []string{"GET"}, index)
	authR.Handle(imageRoute, []string{"GET"}, image)
	authR.Handle(audioRoute, []string{"GET"}, audio)
	authR.Handle(regexp.MustCompile(`^/search$`), []string{"GET"}, ss)
	authR.Handle(regexp.MustCompile(`^/calls$`), []string{"GET"}, cls)
	authR.Handle(regexp.MustCompile(`^/conferences$`), []string{"GET"}, confs)
	authR.Handle(regexp.MustCompile(`^/messages$`), []string{"GET"}, mls)
	authR.Handle(regexp.MustCompile(`^/alerts$`), []string{"GET"}, als)
	authR.Handle(regexp.MustCompile(`^/tz$`), []string{"POST"}, tz)
	authR.Handle(conferenceInstanceRoute, []string{"GET"}, confInstance)
	authR.Handle(callInstanceRoute, []string{"GET"}, cis)
	authR.Handle(messageInstanceRoute, []string{"GET"}, mis)
	authH := AddAuthenticator(authR, ls, settings.Authenticator)
	authH = handlers.WithLogger(authH, settings.Logger)
	if len(settings.IPSubnets) > 0 {
		authH = whitelistIPs(authH, settings.Logger, settings.IPSubnets)
	}

	r := new(handlers.Regexp)
	// TODO - don't protect static routes with basic auth
	r.Handle(regexp.MustCompile(`(^/static|^/favicon.ico$)`), []string{"GET"}, handlers.GZip(staticServer))
	r.Handle(regexp.MustCompile(`^/opensearch.xml$`), []string{"GET"}, o)
	r.Handle(regexp.MustCompile(`^/auth/logout$`), []string{"POST"}, logout)
	// todo awkward using HTTP methods here
	r.Handle(regexp.MustCompile(`^/`), []string{"GET", "POST", "PUT", "DELETE"}, authH)
	h := UpgradeInsecureHandler(r, settings.AllowUnencryptedTraffic)

	// Innermost handlers are first.
	h = handlers.Server(h, "logrole/"+Version)
	h = handlers.UUID(h)
	h = handlers.TrailingSlashRedirect(h)
	h = handlers.Debug(h)
	h = handlers.WithTimeout(h, 32*time.Second)
	h = settings.Reporter.ReportPanics(h)
	h = handlers.Duration(h)
	return &Server{
		Handler:  h,
		PageSize: settings.PageSize,
		vc:       vc,
		DoneChan: make(chan bool, 1),
	}, nil
}
