package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

const callPattern = `(?P<sid>CA[a-f0-9]{32})`

var callInstanceRoute = regexp.MustCompile("^/calls/" + callPattern + "$")

type callListServer struct {
	log.Logger
	Client         views.Client
	LocationFinder services.LocationFinder
	PageSize       uint
	MaxResourceAge time.Duration
	secretKey      *[32]byte
	tpl            *template.Template
}

func newCallListServer(l log.Logger, vc views.Client, lf services.LocationFinder,
	pageSize uint, maxResourceAge time.Duration,
	secretKey *[32]byte) (*callListServer, error) {
	cs := &callListServer{
		Logger:         l,
		Client:         vc,
		LocationFinder: lf,
		PageSize:       pageSize,
		MaxResourceAge: maxResourceAge,
		secretKey:      secretKey,
	}
	tpl, err := newTpl(template.FuncMap{
		"is_our_pn": vc.IsTwilioNumber,
		"min":       minFunc(cs.MaxResourceAge),
		"max":       maxLoc,
		"start_val": cs.StartSearchVal,
		"end_val":   cs.EndSearchVal,
	}, base+callListTpl+pagingTpl+phoneTpl+copyScript)
	if err != nil {
		return nil, err
	}
	cs.tpl = tpl
	return cs, nil
}

type callInstanceServer struct {
	log.Logger
	Client         views.Client
	LocationFinder services.LocationFinder
	tpl            *template.Template
}

func newCallInstanceServer(l log.Logger, vc views.Client,
	lf services.LocationFinder) (*callInstanceServer, error) {
	c := &callInstanceServer{
		Logger:         l,
		Client:         vc,
		LocationFinder: lf,
	}
	tpl, err := newTpl(template.FuncMap{
		"is_our_pn": vc.IsTwilioNumber,
	}, base+callInstanceTpl+recordingTpl+phoneTpl+sidTpl+copyScript)
	if err != nil {
		return nil, err
	}
	c.tpl = tpl
	return c, nil
}

type callInstanceData struct {
	Call       *views.Call
	Loc        *time.Location
	Recordings *recordingResp
	AlertError error
	Alerts     *views.AlertPage
}

type callListData struct {
	Page                  *views.CallPage
	EncryptedPreviousPage string
	EncryptedNextPage     string
	Loc                   *time.Location
	Query                 url.Values
	Err                   string
}

func (c *callListData) Title() string {
	return "Calls"
}

func (c *callListData) Path() string {
	return "/calls"
}

func (c *callListData) NextQuery() template.URL {
	data := url.Values{}
	if c.EncryptedNextPage != "" {
		data.Set("next", c.EncryptedNextPage)
	}
	if start, ok := c.Query["start-after"]; ok {
		data.Set("start-after", start[0])
	}
	if end, ok := c.Query["start-before"]; ok {
		data.Set("start-before", end[0])
	}
	return template.URL(data.Encode())
}

func (c *callListData) PreviousQuery() template.URL {
	data := url.Values{}
	if c.EncryptedPreviousPage != "" {
		data.Set("next", c.EncryptedPreviousPage)
	}
	if start, ok := c.Query["start-after"]; ok {
		data.Set("start-after", start[0])
	}
	if end, ok := c.Query["start-before"]; ok {
		data.Set("start-before", end[0])
	}
	return template.URL(data.Encode())
}

func (s *callListServer) StartSearchVal(query url.Values, loc *time.Location) string {
	if start, ok := query["start-after"]; ok {
		return start[0]
	}
	if s.MaxResourceAge == config.DefaultMaxResourceAge {
		// one week ago, arbitrary
		return minLoc(7*24*time.Hour, loc)
	} else {
		return minLoc(s.MaxResourceAge, loc)
	}
}

func (s *callListServer) EndSearchVal(query url.Values, loc *time.Location) string {
	if end, ok := query["start-before"]; ok {
		return end[0]
	}
	return maxLoc(loc)
}

func (s *callListServer) validParams() []string {
	return []string{"from", "to", "next", "start-after", "start-before"}
}

func (s *callListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewCalls() {
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
	loc := s.LocationFinder.GetLocationReq(r)
	// We always set startTime and endTime on the request, though they may end
	// up just being sentinels
	startTime, endTime, wroteError := getTimes(w, r, "start-after", "start-before", loc, query, s)
	if wroteError {
		return
	}
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	var err error
	next, nextErr := getNext(query, s.secretKey)
	if nextErr != nil {
		err = errors.New("Could not decrypt `next` query parameter: " + nextErr.Error())
		s.renderError(w, r, http.StatusBadRequest, query, err)
		return
	}
	page := new(views.CallPage)
	cachedAt := time.Time{}
	queryStart := time.Now()
	if next != "" {
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			s.Warn("Invalid next page URI", "next", next, "opaque", query.Get("next"))
			s.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, cachedAt, err = s.Client.GetNextCallPageInRange(ctx, u, startTime, endTime, next)
		setNextPageValsOnQuery(next, query)
	} else {
		// valid values: https://www.twilio.com/docs/api/rest/call#list
		data := url.Values{}
		data.Set("PageSize", strconv.FormatUint(uint64(s.PageSize), 10))
		if filterErr := setPageFilters(query, data); filterErr != nil {
			s.renderError(w, r, http.StatusBadRequest, query, filterErr)
			return
		}
		page, cachedAt, err = s.Client.GetCallPageInRange(ctx, u, startTime, endTime, data)
	}
	if err == twilio.NoMoreResults {
		page = new(views.CallPage)
		err = nil
	}
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
	// Fetch the next page into the cache
	go func(u *config.User, n types.NullString, startTime, endTime time.Time) {
		if n.Valid {
			if _, _, err := s.Client.GetNextCallPageInRange(context.Background(), u, startTime, endTime, n.String); err != nil {
				s.Debug("Error fetching next page", "err", err)
			}
		}
	}(u, page.NextPageURI(), startTime, endTime)
	data := &baseData{
		LF:       s.LocationFinder,
		CachedAt: cachedAt,
		Duration: time.Since(queryStart),
	}
	data.Data = &callListData{
		Page:                  page,
		Loc:                   loc,
		Query:                 query,
		EncryptedNextPage:     getEncryptedPage(page.NextPageURI(), s.secretKey),
		EncryptedPreviousPage: getEncryptedPage(page.PreviousPageURI(), s.secretKey),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

func (c *callListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	str := cleanError(err)
	data := &baseData{
		LF: c.LocationFinder,
		Data: &callListData{
			Err:   str,
			Loc:   c.LocationFinder.GetLocationReq(r),
			Query: query,
			Page:  new(views.CallPage),
		},
	}
	if code >= 500 {
		c.Error("Error responding to request", "status", code, "url", r.URL.String(), "err", err)
	} else {
		c.Warn("Error responding to request", "status", code, "url", r.URL.String(), "err", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := render(w, r, c.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
		return
	}
}

func (c *callInstanceData) Title() string {
	return "Call Details"
}

type recordingResp struct {
	Err                  error
	Recordings           []*views.Recording
	CanPlayRecording     bool
	CanViewNumRecordings bool
}

func (c *callInstanceServer) fetchRecordings(ctx context.Context, sid string, u *config.User, rch chan<- *recordingResp) {
	defer close(rch)
	if u.CanViewNumRecordings() == false {
		rch <- &recordingResp{
			Err:                  config.PermissionDenied,
			CanViewNumRecordings: false,
		}
		return
	}
	rp, err := c.Client.GetCallRecordings(ctx, u, sid, nil)
	if err != nil {
		rch <- &recordingResp{Err: err}
		return
	}
	rs := rp.Recordings()
	uri := rp.NextPageURI()
	for uri.Valid {
		rp, err := c.Client.GetNextRecordingPage(ctx, u, uri.String)
		if err == twilio.NoMoreResults {
			break
		}
		if err != nil {
			rch <- &recordingResp{Err: err}
			return
		}
		rs = append(rs, rp.Recordings()...)
	}
	canPlayRecording := false
	for _, recording := range rs {
		if recording.CanPlay() {
			canPlayRecording = true
			break
		}
	}
	rch <- &recordingResp{
		Recordings:           rs,
		CanPlayRecording:     canPlayRecording,
		CanViewNumRecordings: u.CanViewNumRecordings(),
	}
}

func (c *callInstanceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewCalls() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
		return
	}
	sid := callInstanceRoute.FindStringSubmatch(r.URL.Path)[1]
	rch := make(chan *recordingResp, 1)
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	go c.fetchRecordings(ctx, sid, u, rch)
	var wg sync.WaitGroup
	wg.Add(1)
	var alerts *views.AlertPage
	var alertsErr error
	go func() {
		alerts, alertsErr = c.Client.GetCallAlerts(ctx, u, sid)
		wg.Done()
	}()
	call, err := c.Client.GetCall(ctx, u, sid)
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
	wg.Wait()
	data := &baseData{
		LF:       c.LocationFinder,
		Duration: time.Since(start),
	}
	cid := &callInstanceData{
		Call:       call,
		Loc:        c.LocationFinder.GetLocationReq(r),
		AlertError: alertsErr,
		Alerts:     alerts,
	}
	if u.CanViewNumRecordings() {
		r := <-rch
		cid.Recordings = r
	}
	data.Data = cid
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render(w, r, c.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
		return
	}
}
