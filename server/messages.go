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
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
	"golang.org/x/net/context"
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
	s := &messageInstanceServer{
		Logger:             l,
		Client:             vc,
		LocationFinder:     lf,
		ShowMediaByDefault: smbd,
	}
	tpl, err := newTpl(template.FuncMap{
		"is_our_pn": vc.IsTwilioNumber,
	}, base+messageInstanceTpl+phoneTpl+sidTpl+copyScript)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
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
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	rch := make(chan *mediaResp, 1)
	go func(sid string) {
		urls, err := s.Client.GetMediaURLs(ctx, u, sid)
		rch <- &mediaResp{
			URLs: urls,
			Err:  err,
		}
		close(rch)
	}(sid)
	message, err := s.Client.GetMessage(ctx, u, sid)
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

func (s *messageListServer) StartSearchVal(query url.Values, loc *time.Location) string {
	if start, ok := query["start"]; ok {
		return start[0]
	}
	if s.MaxResourceAge == config.DefaultMaxResourceAge {
		// one week ago, arbitrary
		return minLoc(7*24*time.Hour, loc)
	} else {
		return minLoc(s.MaxResourceAge, loc)
	}
}

func (s *messageListServer) EndSearchVal(query url.Values, loc *time.Location) string {
	if end, ok := query["end"]; ok {
		return end[0]
	}
	return maxLoc(loc)
}

func newMessageListServer(l log.Logger, vc views.Client, lf services.LocationFinder, pageSize uint, maxResourceAge time.Duration, secretKey *[32]byte) (*messageListServer, error) {
	s := &messageListServer{
		Logger:         l,
		Client:         vc,
		LocationFinder: lf,
		PageSize:       pageSize,
		MaxResourceAge: maxResourceAge,
		secretKey:      secretKey,
	}
	tpl, err := newTpl(template.FuncMap{
		"is_our_pn": vc.IsTwilioNumber,
		"min":       minFunc(s.MaxResourceAge),
		"max":       maxLoc,
		"start_val": s.StartSearchVal,
		"end_val":   s.EndSearchVal,
	}, base+messageListTpl+messageStatusTpl+pagingTpl+phoneTpl+copyScript)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
	return s, nil
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

func (m *messageListData) NextQuery() template.URL {
	data := url.Values{}
	if m.EncryptedNextPage != "" {
		data.Set("next", m.EncryptedNextPage)
	}
	if end, ok := m.Query["end"]; ok {
		data.Set("end", end[0])
	}
	if start, ok := m.Query["start"]; ok {
		data.Set("start", start[0])
	}
	return template.URL(data.Encode())
}

func (m *messageListData) PreviousQuery() template.URL {
	data := url.Values{}
	if m.EncryptedPreviousPage != "" {
		data.Set("next", m.EncryptedPreviousPage)
	}
	if end, ok := m.Query["end"]; ok {
		data.Set("end", end[0])
	}
	if start, ok := m.Query["start"]; ok {
		data.Set("start", start[0])
	}
	return template.URL(data.Encode())
}

func (s *messageListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &baseData{LF: s.LocationFinder,
		Data: &messageListData{
			Err:            str,
			Loc:            s.LocationFinder.GetLocationReq(r),
			Query:          query,
			Page:           new(views.MessagePage),
			MaxResourceAge: s.MaxResourceAge,
		}}
	if code >= 500 {
		s.Error("Error responding to request", "status", code, "url", r.URL.String(), "err", err)
	} else {
		s.Warn("Error responding to request", "status", code, "url", r.URL.String(), "err", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

func (s *messageListServer) validParams() []string {
	return []string{"start", "end", "next", "to", "from"}
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
	if err := validateParams(s.validParams(), query); err != nil {
		s.renderError(w, r, http.StatusBadRequest, query, err)
		return
	}
	page := new(views.MessagePage)
	loc := s.LocationFinder.GetLocationReq(r)
	var err error
	startTime, endTime, wroteError := getTimes(w, r, "start", "end", loc, query, s)
	if wroteError {
		return
	}
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	next, nextErr := getNext(query, s.secretKey)
	if nextErr != nil {
		err = errors.New("Could not decrypt `next` query parameter: " + nextErr.Error())
		s.renderError(w, r, http.StatusBadRequest, query, err)
		return
	}
	cachedAt := time.Time{}
	start := time.Now()
	if next != "" {
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			s.Warn("Invalid next page URI", "next", next, "opaque", query.Get("next"))
			s.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, cachedAt, err = s.Client.GetNextMessagePageInRange(ctx, u, startTime, endTime, next)
		setNextPageValsOnQuery(next, query)
	} else {
		// valid values: https://www.twilio.com/docs/api/rest/message#list
		data := url.Values{}
		data.Set("PageSize", strconv.FormatUint(uint64(s.PageSize), 10))
		if filterErr := setPageFilters(query, data); filterErr != nil {
			s.renderError(w, r, http.StatusBadRequest, query, filterErr)
			return
		}
		page, cachedAt, err = s.Client.GetMessagePageInRange(ctx, u, startTime, endTime, data)
	}
	if err == twilio.NoMoreResults {
		page = new(views.MessagePage)
		err = nil
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
	go func(u *config.User, n types.NullString, start, end time.Time) {
		if n.Valid {
			if _, _, err := s.Client.GetNextMessagePageInRange(context.Background(), u, start, end, n.String); err != nil {
				s.Debug("Error fetching next page", "err", err)
			}
		}
	}(u, page.NextPageURI(), startTime, endTime)
	data := &baseData{
		LF:       s.LocationFinder,
		Duration: time.Since(start),
		CachedAt: cachedAt,
		Data: &messageListData{
			Page:                  page,
			Loc:                   loc,
			Query:                 query,
			MaxResourceAge:        s.MaxResourceAge,
			EncryptedPreviousPage: getEncryptedPage(page.PreviousPageURI(), s.secretKey),
			EncryptedNextPage:     getEncryptedPage(page.NextPageURI(), s.secretKey),
		}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render(w, r, s.tpl, "base", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
}
