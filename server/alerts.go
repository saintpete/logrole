package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"sort"
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

const alertPattern = `(?P<sid>NO[a-f0-9]{32})`

var alertInstanceRoute = regexp.MustCompile("^/alerts/" + alertPattern + "$")

var validAlertLevels = []twilio.LogLevel{
	twilio.LogLevelError,
	twilio.LogLevelWarning,
	twilio.LogLevelNotice,
	twilio.LogLevelDebug,
}

type alertInstanceServer struct {
	log.Logger
	Client         views.Client
	LocationFinder services.LocationFinder
	tpl            *template.Template
}

func halve(firstHalf bool, vals url.Values) map[string]string {
	switch len(vals) {
	case 0:
		return map[string]string{}
	case 1:
		if firstHalf {
			m := make(map[string]string, 1)
			for k, v := range vals {
				m[k] = v[0]
			}
			return m
		} else {
			return map[string]string{}
		}
	default:
		keys := make([]string, len(vals))
		i := 0
		for k := range vals {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		split := (len(vals) + 1) / 2
		var keyHalf []string
		if firstHalf {
			keyHalf = keys[:split]
		} else {
			keyHalf = keys[split:]
		}
		d := make(map[string]string, len(keyHalf))
		for _, k := range keyHalf {
			d[k] = vals.Get(k)
		}
		return d
	}
}

func newAlertInstanceServer(l log.Logger, vc views.Client, lf services.LocationFinder) (*alertInstanceServer, error) {
	tpl, err := newTpl(template.FuncMap{
		"has_prefix":  strings.HasPrefix,
		"status_text": http.StatusText,
		"halve":       halve,
	}, base+alertInstanceTpl+sidTpl)
	if err != nil {
		return nil, err
	}
	return &alertInstanceServer{
		Logger:         l,
		Client:         vc,
		LocationFinder: lf,
		tpl:            tpl,
	}, nil
}

type alertInstanceData struct {
	Alert *views.Alert
	Loc   *time.Location
}

func (a *alertInstanceData) Title() string {
	return "Alert Details"
}

func (s *alertInstanceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewAlerts() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
		return
	}
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	sid := alertInstanceRoute.FindStringSubmatch(r.URL.Path)[1]
	start := time.Now()
	alert, err := s.Client.GetAlert(ctx, u, sid)
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
	data := &baseData{
		LF:       s.LocationFinder,
		Duration: time.Since(start),
		Data: &alertInstanceData{
			Alert: alert,
			Loc:   s.LocationFinder.GetLocationReq(r),
		},
	}
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

type alertListServer struct {
	log.Logger
	Client         views.Client
	PageSize       uint
	MaxResourceAge time.Duration
	LocationFinder services.LocationFinder
	secretKey      *[32]byte
	tpl            *template.Template
}

type alertListData struct {
	Page                  *views.AlertPage
	EncryptedNextPage     string
	EncryptedPreviousPage string
	Loc                   *time.Location
	Query                 url.Values
	Err                   string
	Freq                  []*alertFrequency
}

func (ad *alertListData) Title() string {
	return "Alerts"
}

func (ad *alertListData) Path() string {
	return "/alerts"
}

func (d *alertListData) LogLevels() []twilio.LogLevel {
	return validAlertLevels
}

func (c *alertListData) NextQuery() template.URL {
	data := url.Values{}
	if c.EncryptedNextPage != "" {
		data.Set("next", c.EncryptedNextPage)
	}
	if start, ok := c.Query["alert-start"]; ok {
		data.Set("alert-start", start[0])
	}
	if end, ok := c.Query["alert-end"]; ok {
		data.Set("alert-end", end[0])
	}
	return template.URL(data.Encode())
}

func (c *alertListData) PreviousQuery() template.URL {
	data := url.Values{}
	if c.EncryptedPreviousPage != "" {
		data.Set("next", c.EncryptedPreviousPage)
	}
	if start, ok := c.Query["alert-start"]; ok {
		data.Set("alert-start", start[0])
	}
	if end, ok := c.Query["alert-end"]; ok {
		data.Set("alert-end", end[0])
	}
	return template.URL(data.Encode())
}

type alertFrequency struct {
	Since    time.Duration
	Name     string
	Count    uint
	HaveMore bool
}

func getAlertFrequency(alerts []*views.Alert, name string, since time.Duration) *alertFrequency {
	now := time.Now()
	count := uint(0)
	for _, alert := range alerts {
		createdAt, err := alert.DateCreated()
		if err != nil {
			continue
		}
		if createdAt.Valid && now.Sub(createdAt.Time) < since {
			count++
		}
	}
	return &alertFrequency{
		Name:     name,
		Count:    count,
		Since:    since,
		HaveMore: count > 0 && int(count) >= len(alerts),
	}
}

func newAlertListServer(l log.Logger, vc views.Client,
	lf services.LocationFinder, pageSize uint, maxResourceAge time.Duration,
	secretKey *[32]byte) (*alertListServer, error) {
	s := &alertListServer{
		Logger:         l,
		Client:         vc,
		PageSize:       pageSize,
		LocationFinder: lf,
		MaxResourceAge: maxResourceAge,
		secretKey:      secretKey,
	}
	tpl, err := newTpl(template.FuncMap{
		"min":        minFunc(s.MaxResourceAge),
		"max":        maxLoc,
		"has_prefix": strings.HasPrefix,
		"start_val":  s.StartSearchVal,
		"end_val":    s.EndSearchVal,
	}, base+alertListTpl+pagingTpl)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
	return s, nil
}

func (s *alertListServer) StartSearchVal(query url.Values, loc *time.Location) string {
	if start, ok := query["alert-start"]; ok {
		return start[0]
	}
	if s.MaxResourceAge == config.DefaultMaxResourceAge {
		// one week ago, arbitrary
		return minLoc(7*24*time.Hour, loc)
	} else {
		return minLoc(s.MaxResourceAge, loc)
	}
}

func (s *alertListServer) EndSearchVal(query url.Values, loc *time.Location) string {
	if end, ok := query["alert-end"]; ok {
		return end[0]
	}
	return maxLoc(loc)
}

func (s *alertListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &baseData{
		LF: s.LocationFinder,
		Data: &alertListData{
			Err:   str,
			Loc:   s.LocationFinder.GetLocationReq(r),
			Query: query,
			Page:  new(views.AlertPage),
		},
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
		return
	}
}

func (s *alertListServer) validParams() []string {
	return []string{"log-level", "resource-sid", "next", "alert-start", "alert-end"}
}

func (s *alertListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewAlerts() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
		return
	}
	query := r.URL.Query()
	if err := validateParams(s.validParams(), query); err != nil {
		s.renderError(w, r, http.StatusBadRequest, query, err)
		return
	}
	loc := s.LocationFinder.GetLocationReq(r)
	// We always set startTime and endTime on the request, though they may end
	// up just being sentinels
	startTime, endTime, wroteError := getTimes(w, r, "alert-start", "alert-end", loc, query, s)
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
	page := new(views.AlertPage)
	cachedAt := time.Time{}
	start := time.Now()
	if next != "" {
		if !strings.HasPrefix(next, twilio.MonitorBaseURL) {
			s.Warn("Invalid next page URI", "next", next, "opaque", query.Get("next"))
			s.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, cachedAt, err = s.Client.GetNextAlertPageInRange(ctx, u, startTime, endTime, next)
		setNextPageValsOnQuery(next, query)
	} else {
		vals := url.Values{}
		vals.Set("PageSize", strconv.FormatUint(uint64(s.PageSize), 10))
		if filterErr := setPageFilters(query, vals); filterErr != nil {
			s.renderError(w, r, http.StatusBadRequest, query, filterErr)
			return
		}
		page, cachedAt, err = s.Client.GetAlertPageInRange(ctx, u, startTime, endTime, vals)
	}
	if err == twilio.NoMoreResults {
		page = new(views.AlertPage)
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
			if _, _, err := s.Client.GetNextAlertPageInRange(context.Background(), u, start, end, n.String); err != nil {
				s.Debug("Error fetching next page", "err", err)
			}
		}
	}(u, page.NextPageURI(), startTime, endTime)
	data := &baseData{
		LF:       s.LocationFinder,
		CachedAt: cachedAt,
		Duration: time.Since(start),
	}
	ad := &alertListData{
		Page:                  page,
		Query:                 query,
		Loc:                   s.LocationFinder.GetLocationReq(r),
		EncryptedNextPage:     getEncryptedPage(page.NextPageURI(), s.secretKey),
		EncryptedPreviousPage: getEncryptedPage(page.PreviousPageURI(), s.secretKey),
	}
	if next == "" {
		alerts := page.Alerts()
		freq := []*alertFrequency{
			getAlertFrequency(alerts, "5 minutes", 5*time.Minute),
			getAlertFrequency(alerts, "hour", time.Hour),
			getAlertFrequency(alerts, "day", 24*time.Hour),
			getAlertFrequency(alerts, "3 days", 24*time.Hour),
		}
		ad.Freq = freq
	}
	data.Data = ad
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}
