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
		Client:         vc,
		PageSize:       pageSize,
		LocationFinder: lf,
		MaxResourceAge: maxResourceAge,
		secretKey:      secretKey,
	}
	tpl, err := newTpl(template.FuncMap{
		"min": minFunc(s.MaxResourceAge),
		"max": max,
	}, base+conferenceListTpl+pagingTpl)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
	return s, nil
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
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	query := r.URL.Query()
	page := new(views.ConferencePage)
	var err error
	opaqueNext := query.Get("next")
	start := time.Now()
	if opaqueNext != "" {
		next, nextErr := services.Unopaque(opaqueNext, c.secretKey)
		if nextErr != nil {
			err = errors.New("Could not decrypt `next` query parameter: " + nextErr.Error())
			c.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			c.Warn("Invalid next page URI", "next", next, "opaque", opaqueNext)
			c.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, err = c.Client.GetNextConferencePage(ctx, u, next)
		setNextPageValsOnQuery(next, query)
	} else {
		data := url.Values{}
		data.Set("PageSize", strconv.FormatUint(uint64(c.PageSize), 10))
		if filterErr := setPageFilters(query, data); filterErr != nil {
			c.renderError(w, r, http.StatusBadRequest, query, filterErr)
			return
		}
		page, err = c.Client.GetConferencePage(ctx, u, data)
	}
	// Fetch the next page into the cache
	go func(u *config.User, n types.NullString) {
		if n.Valid {
			if _, err := c.Client.GetNextConferencePage(context.Background(), u, n.String); err != nil {
				c.Debug("Error fetching next page", "err", err)
			}
		}
	}(u, page.NextPageURI())
	if err != nil {
		rest.ServerError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = render(w, r, c.tpl, "base", &baseData{
		LF:       c.LocationFinder,
		Duration: time.Since(start),
		Data: &conferenceListData{
			Query:                 r.URL.Query(),
			Page:                  page,
			Loc:                   c.LocationFinder.GetLocationReq(r),
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
