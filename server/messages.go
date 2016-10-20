package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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

var messageInstanceTemplate *template.Template
var messageListTemplate *template.Template

const messagePattern = `(?P<sid>(MM|SM)[a-f0-9]{32})`

var messageInstanceRoute = regexp.MustCompile("^/messages/" + messagePattern + "$")

func init() {
	base := string(assets.MustAsset("templates/base.html"))
	templates := template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base))

	tlist := template.Must(templates.Clone())
	listTpl := string(assets.MustAsset("templates/messages/list.html"))
	messageListTemplate = template.Must(tlist.Parse(listTpl))

	tinstance := template.Must(templates.Clone())
	instanceTpl := string(assets.MustAsset("templates/messages/instance.html"))
	messageInstanceTemplate = template.Must(tinstance.Parse(instanceTpl))
}

type messageInstanceServer struct {
	Client             *views.Client
	Location           *time.Location
	ShowMediaByDefault bool
}

type messageInstanceData struct {
	baseData
	Message            *views.Message
	Loc                *time.Location
	Media              *mediaResp
	ShowMediaByDefault bool
}

func (m *messageInstanceData) Title() string {
	return "Message Details"
}

type mediaResp struct {
	Err  error
	URLs []*url.URL
}

func (s *messageInstanceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	sid := messageInstanceRoute.FindStringSubmatch(r.URL.Path)[1]
	start := time.Now()
	rch := make(chan *mediaResp, 1)
	go func(sid string) {
		urls, err := s.Client.GetMediaURLs(u, sid)
		rch <- &mediaResp{
			URLs: urls,
			Err:  err,
		}
		close(rch)
	}(sid)
	message, err := s.Client.GetMessage(u, sid)
	switch err {
	case nil:
		break
	case config.PermissionDenied, config.ErrTooOld:
		rest.Forbidden(w, r, &rest.Error{Title: err.Error()})
		return
	default:
		rest.ServerError(w, r, err)
		return
	}
	if !message.CanViewProperty("Sid") {
		rest.Forbidden(w, r, &rest.Error{Title: "Cannot view this message"})
		return
	}
	data := &messageInstanceData{
		Message:            message,
		Loc:                s.Location,
		ShowMediaByDefault: s.ShowMediaByDefault,
	}
	numMedia, err := message.NumMedia()
	switch {
	case err != nil:
		data.Media = &mediaResp{Err: err}
	case numMedia > 0:
		r := <-rch
		data.Media = r
	}
	data.Duration = time.Since(start)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data.Start = time.Now()
	if err := render(w, messageInstanceTemplate, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

type messageListServer struct {
	Client         *views.Client
	Location       *time.Location
	PageSize       uint
	SecretKey      *[32]byte
	MaxResourceAge time.Duration
}

type messageData struct {
	baseData
	Page              *views.MessagePage
	EncryptedNextPage string
	Loc               *time.Location
	Query             url.Values
	Err               string
	MaxResourceAge    time.Duration
}

func (m *messageData) Title() string {
	return "Messages"
}

// Min returns the minimum acceptable resource date, formatted for use in a
// date HTML input field.
func (m *messageData) Min() string {
	return time.Now().Add(-m.MaxResourceAge).Format("2006-01-02")
}

// Max returns a the maximum acceptable resource date, formatted for use in a
// date HTML input field.
func (m *messageData) Max() string {
	return time.Now().UTC().Format("2006-01-02")
}

func (s *messageListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &messageData{
		Err:            str,
		Query:          query,
		Page:           new(views.MessagePage),
		MaxResourceAge: s.MaxResourceAge,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	data.Start = time.Now()
	if err := render(w, messageListTemplate, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

func (s *messageListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	// This is modified as we parse the query; specifically we add some values
	// if they are present in the next page URI.
	query := r.URL.Query()
	page := new(views.MessagePage)
	var err error
	opaqueNext := query.Get("next")
	start := time.Now()
	if opaqueNext != "" {
		next, err := services.Unopaque(opaqueNext, s.SecretKey)
		if err != nil {
			err = errors.New("Could not decrypt `next` query parameter: " + err.Error())
			s.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			handlers.Logger.Warn("Invalid next page URI", "next", next, "opaque", opaqueNext)
			s.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, err = s.Client.GetNextMessagePage(u, next)
		setNextPageValsOnQuery(next, query)
	} else {
		// valid values: https://www.twilio.com/docs/api/rest/message#list
		data := url.Values{}
		data.Set("PageSize", strconv.FormatUint(uint64(s.PageSize), 10))
		if err := setPageFilters(query, data); err != nil {
			s.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		page, err = s.Client.GetMessagePage(u, data)
	}
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
	data := &messageData{
		Page:           page,
		Loc:            s.Location,
		Query:          query,
		MaxResourceAge: s.MaxResourceAge,
	}
	data.Duration = time.Since(start)
	data.EncryptedNextPage, err = getEncryptedNextPage(page.NextPageURI(), s.SecretKey)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data.Start = time.Now()
	if err := render(w, messageListTemplate, "base", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
}
