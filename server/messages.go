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

	log "github.com/inconshreveable/log15"
	types "github.com/kevinburke/go-types"
	"github.com/kevinburke/rest"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
)

const messagePattern = `(?P<sid>(MM|SM)[a-f0-9]{32})`

var messageInstanceRoute = regexp.MustCompile("^/messages/" + messagePattern + "$")

type messageInstanceServer struct {
	log.Logger
	Client             views.Client
	LocationFinder     services.LocationFinder
	ShowMediaByDefault bool
	tpl                *template.Template
}

func newMessageInstanceServer(l log.Logger, vc views.Client, lf services.LocationFinder, smbd bool) (*messageInstanceServer, error) {
	base, err := assets.AssetString("templates/base.html")
	if err != nil {
		return nil, err
	}
	phoneTpl, err := assets.AssetString("templates/snippets/phonenumber.html")
	if err != nil {
		return nil, err
	}
	copyScript, err := assets.AssetString("templates/snippets/copy-phonenumber.js")
	if err != nil {
		return nil, err
	}
	sidTpl, err := assets.AssetString("templates/snippets/sid.html")
	if err != nil {
		return nil, err
	}
	instanceTpl, err := assets.AssetString("templates/messages/instance.html")
	if err != nil {
		return nil, err
	}
	s := &messageInstanceServer{
		Logger:             l,
		Client:             vc,
		LocationFinder:     lf,
		ShowMediaByDefault: smbd,
	}
	templates, err := template.New("base").Option("missingkey=error").Funcs(funcMap).Funcs(template.FuncMap{
		"is_our_pn": vc.IsTwilioNumber,
	}).Parse(base + instanceTpl + phoneTpl + sidTpl + copyScript)
	if err != nil {
		return nil, err
	}
	s.tpl = templates
	return s, nil
}

type messageInstanceData struct {
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
		switch terr := err.(type) {
		case *rest.Error:
			switch terr.StatusCode {
			case 404:
				rest.NotFound(w, r)
			default:
				rest.ServerError(w, r, terr)
			}
		default:
			rest.ServerError(w, r, err)
		}
		return
	}
	if !message.CanViewProperty("Sid") {
		rest.Forbidden(w, r, &rest.Error{Title: "Cannot view this message"})
		return
	}
	baseData := &baseData{LF: s.LocationFinder, Duration: time.Since(start)}
	data := &messageInstanceData{
		Message:            message,
		Loc:                s.LocationFinder.GetLocationReq(r),
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
	baseData.Data = data
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render(w, r, s.tpl, "base", baseData); err != nil {
		rest.ServerError(w, r, err)
	}
}

type messageListServer struct {
	log.Logger
	Client         views.Client
	LocationFinder services.LocationFinder
	PageSize       uint
	secretKey      *[32]byte
	MaxResourceAge time.Duration
	tpl            *template.Template
}

func newMessageListServer(l log.Logger, c views.Client, lf services.LocationFinder, pageSize uint, maxResourceAge time.Duration, secretKey *[32]byte) (*messageListServer, error) {
	base, err := assets.AssetString("templates/base.html")
	if err != nil {
		return nil, err
	}
	listTpl, err := assets.AssetString("templates/messages/list.html")
	if err != nil {
		return nil, err
	}
	phoneTpl, err := assets.AssetString("templates/snippets/phonenumber.html")
	if err != nil {
		return nil, err
	}
	pagingTpl, err := assets.AssetString("templates/snippets/paging.html")
	if err != nil {
		return nil, err
	}
	copyScript, err := assets.AssetString("templates/snippets/copy-phonenumber.js")
	if err != nil {
		return nil, err
	}
	s := &messageListServer{
		Logger:         l,
		Client:         c,
		LocationFinder: lf,
		PageSize:       pageSize,
		MaxResourceAge: maxResourceAge,
		secretKey:      secretKey,
	}
	templates, err := template.New("base").Option("missingkey=error").Funcs(funcMap).Funcs(template.FuncMap{
		"is_our_pn": s.isTwilioNumber,
	}).Parse(base + listTpl + pagingTpl + phoneTpl + copyScript)
	if err != nil {
		return nil, err
	}
	s.tpl = templates
	return s, nil
}

func (s *messageListServer) isTwilioNumber(num twilio.PhoneNumber) bool {
	return s.Client.IsTwilioNumber(num)
}

type messageListData struct {
	Page                  *views.MessagePage
	EncryptedPreviousPage string
	EncryptedNextPage     string
	Loc                   *time.Location
	Query                 url.Values
	Err                   string
	MaxResourceAge        time.Duration
}

func (m *messageListData) Title() string {
	return "Messages"
}

func (m *messageListData) Path() string {
	return "/messages"
}

// Min returns the minimum acceptable resource date, formatted for use in a
// date HTML input field.
func (m *messageListData) Min() string {
	return time.Now().Add(-m.MaxResourceAge).Format("2006-01-02")
}

// Max returns a the maximum acceptable resource date, formatted for use in a
// date HTML input field.
func (m *messageListData) Max() string {
	return time.Now().UTC().Format("2006-01-02")
}

func (s *messageListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &baseData{LF: s.LocationFinder,
		Data: &messageListData{
			Err:            str,
			Query:          query,
			Page:           new(views.MessagePage),
			MaxResourceAge: s.MaxResourceAge,
		}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

func (s *messageListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewMessages() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
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
		next, nextErr := services.Unopaque(opaqueNext, s.secretKey)
		if nextErr != nil {
			err = errors.New("Could not decrypt `next` query parameter: " + nextErr.Error())
			s.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			s.Warn("Invalid next page URI", "next", next, "opaque", opaqueNext)
			s.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, err = s.Client.GetNextMessagePage(u, next)
		setNextPageValsOnQuery(next, query)
	} else {
		// valid values: https://www.twilio.com/docs/api/rest/message#list
		data := url.Values{}
		data.Set("PageSize", strconv.FormatUint(uint64(s.PageSize), 10))
		if filterErr := setPageFilters(query, data); filterErr != nil {
			s.renderError(w, r, http.StatusBadRequest, query, filterErr)
			return
		}
		page, err = s.Client.GetMessagePage(u, data)
	}
	if err != nil {
		switch terr := err.(type) {
		case *rest.Error:
			switch terr.StatusCode {
			case 400:
				s.renderError(w, r, http.StatusBadRequest, query, err)
			case 404:
				rest.NotFound(w, r)
			default:
				rest.ServerError(w, r, terr)
			}
		default:
			rest.ServerError(w, r, err)
		}
		return
	}
	// Fetch the next page into the cache
	go func(u *config.User, n types.NullString) {
		if n.Valid {
			if _, err := s.Client.GetNextMessagePage(u, n.String); err != nil {
				s.Debug("Error fetching next page", "err", err)
			}
		}
	}(u, page.NextPageURI())
	data := &baseData{
		LF: s.LocationFinder,
		Data: &messageListData{
			Page:                  page,
			Loc:                   s.LocationFinder.GetLocationReq(r),
			Query:                 query,
			MaxResourceAge:        s.MaxResourceAge,
			EncryptedPreviousPage: getEncryptedPage(page.PreviousPageURI(), s.secretKey),
			EncryptedNextPage:     getEncryptedPage(page.NextPageURI(), s.secretKey),
		}, Duration: time.Since(start)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render(w, r, s.tpl, "base", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
}
