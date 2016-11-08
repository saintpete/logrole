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

const conferencePattern = `(?P<sid>CF[a-f0-9]{32})`

var conferenceInstanceRoute = regexp.MustCompile("^/conferences/" + conferencePattern + "$")

type conferenceListServer struct {
	log.Logger
	Client         views.Client
	PageSize       uint
	MaxResourceAge time.Duration
	LocationFinder services.LocationFinder
	secretKey      *[32]byte
	tpl            *template.Template
}

type conferenceListData struct {
	Err                   string
	Query                 url.Values
	Page                  *views.ConferencePage
	Loc                   *time.Location
	EncryptedNextPage     string
	EncryptedPreviousPage string
}

func (d *conferenceListData) Title() string {
	return "Conferences"
}

func (d *conferenceListData) Path() string {
	return "/conferences"
}

type conferenceInstanceServer struct {
	log.Logger
	Client         views.Client
	LocationFinder services.LocationFinder
	tpl            *template.Template
}

func newConferenceInstanceServer(l log.Logger, vc views.Client,
	lf services.LocationFinder) (*conferenceInstanceServer, error) {
	c := &conferenceInstanceServer{
		Logger:         l,
		Client:         vc,
		LocationFinder: lf,
	}
	tpl, err := newTpl(template.FuncMap{}, base+conferenceInstanceTpl+sidTpl)
	if err != nil {
		return nil, err
	}
	c.tpl = tpl
	return c, nil
}

// Not putting this in the twilio-go library since Twilio might add more
// statuses later.
var validConferenceStatuses = []twilio.Status{twilio.StatusInProgress, twilio.StatusCompleted}

func (d *conferenceListData) Statuses() []twilio.Status {
	return validConferenceStatuses
}

func newConferenceListServer(l log.Logger, vc views.Client,
	lf services.LocationFinder, pageSize uint, maxResourceAge time.Duration,
	secretKey *[32]byte) (*conferenceListServer, error) {
	s := &conferenceListServer{
		Logger:         l,
		Client:         vc,
		PageSize:       pageSize,
		LocationFinder: lf,
		MaxResourceAge: maxResourceAge,
		secretKey:      secretKey,
	}
	tpl, err := newTpl(template.FuncMap{
		"min":       minFunc(s.MaxResourceAge),
		"max":       maxLoc,
		"start_val": s.StartSearchVal,
		"end_val":   s.EndSearchVal,
	}, base+conferenceListTpl+copyScript+pagingTpl)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
	return s, nil
}

func (c *conferenceListData) NextQuery() template.URL {
	data := url.Values{}
	if c.EncryptedNextPage != "" {
		data.Set("next", c.EncryptedNextPage)
	}
	if end, ok := c.Query["created-before"]; ok {
		data.Set("created-before", end[0])
	}
	if start, ok := c.Query["created-after"]; ok {
		data.Set("created-after", start[0])
	}
	return template.URL(data.Encode())
}

func (c *conferenceListData) PreviousQuery() template.URL {
	data := url.Values{}
	if c.EncryptedPreviousPage != "" {
		data.Set("next", c.EncryptedPreviousPage)
	}
	if end, ok := c.Query["created-before"]; ok {
		data.Set("created-before", end[0])
	}
	if start, ok := c.Query["created-after"]; ok {
		data.Set("created-after", start[0])
	}
	return template.URL(data.Encode())
}

func (s *conferenceListServer) StartSearchVal(query url.Values, loc *time.Location) string {
	if start, ok := query["created-after"]; ok {
		return start[0]
	}
	if s.MaxResourceAge == config.DefaultMaxResourceAge {
		// one week ago, arbitrary
		return minLoc(7*24*time.Hour, loc)
	} else {
		return minLoc(s.MaxResourceAge, loc)
	}
}

func (s *conferenceListServer) EndSearchVal(query url.Values, loc *time.Location) string {
	if end, ok := query["created-before"]; ok {
		return end[0]
	}
	return maxLoc(loc)
}

func (c *conferenceListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &baseData{
		LF: c.LocationFinder,
		Data: &conferenceListData{
			Err:   str,
			Query: query,
			Loc:   c.LocationFinder.GetLocationReq(r),
			Page:  new(views.ConferencePage),
		},
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := render(w, r, c.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
		return
	}
}

func (c *conferenceListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewConferences() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
		return
	}
	var err error
	query := r.URL.Query()
	loc := c.LocationFinder.GetLocationReq(r)
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	// We always set startTime and endTime on the request, though they may end
	// up just being sentinels
	startTime, endTime, wroteError := getTimes(w, r, "created-after", "created-before", loc, query, c)
	if wroteError {
		return
	}
	next, nextErr := getNext(query, c.secretKey)
	if nextErr != nil {
		err = errors.New("Could not decrypt `next` query parameter: " + nextErr.Error())
		c.renderError(w, r, http.StatusBadRequest, query, err)
		return
	}
	page := new(views.ConferencePage)
	cachedAt := time.Time{}
	start := time.Now()
	if next != "" {
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			c.Warn("Invalid next page URI", "next", next, "opaque", query.Get("next"))
			c.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, cachedAt, err = c.Client.GetNextConferencePageInRange(ctx, u, startTime, endTime, next)
		setNextPageValsOnQuery(next, query)
	} else {
		data := url.Values{}
		data.Set("PageSize", strconv.FormatUint(uint64(c.PageSize), 10))
		if filterErr := setPageFilters(query, data); filterErr != nil {
			c.renderError(w, r, http.StatusBadRequest, query, filterErr)
			return
		}
		page, cachedAt, err = c.Client.GetConferencePageInRange(ctx, u, startTime, endTime, data)
	}
	if err == twilio.NoMoreResults {
		page = new(views.ConferencePage)
		err = nil
	}
	if err != nil {
		rest.ServerError(w, r, err)
		return
	}
	// Fetch the next page into the cache
	go func(u *config.User, n types.NullString, start, end time.Time) {
		if n.Valid {
			if _, _, err := c.Client.GetNextConferencePageInRange(context.Background(), u, start, end, n.String); err != nil {
				c.Debug("Error fetching next page", "err", err)
			}
		}
	}(u, page.NextPageURI(), startTime, endTime)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = render(w, r, c.tpl, "base", &baseData{
		LF:       c.LocationFinder,
		CachedAt: cachedAt,
		Duration: time.Since(start),
		Data: &conferenceListData{
			Query:                 r.URL.Query(),
			Page:                  page,
			Loc:                   loc,
			EncryptedNextPage:     getEncryptedPage(page.NextPageURI(), c.secretKey),
			EncryptedPreviousPage: getEncryptedPage(page.PreviousPageURI(), c.secretKey),
		},
	})
	if err != nil {
		rest.ServerError(w, r, err)
	}
}

func (c *conferenceInstanceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewConferences() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
		return
	}
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	sid := conferenceInstanceRoute.FindStringSubmatch(r.URL.Path)[1]
	start := time.Now()
	conference, err := c.Client.GetConference(ctx, u, sid)
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
		LF:       c.LocationFinder,
		Duration: time.Since(start),
		Data: &conferenceInstanceData{
			Conference: conference,
			Loc:        c.LocationFinder.GetLocationReq(r),
		},
	}
	if err := render(w, r, c.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

type conferenceInstanceData struct {
	Conference *views.Conference
	Loc        *time.Location
}

func (c *conferenceInstanceData) Title() string {
	return "Conference Details"
}
