package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/kevinburke/rest"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
)

const conferencePattern = `(?P<sid>CF[a-f0-9]{32})`

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
	query := r.URL.Query()
	data := url.Values{}
	data.Set("PageSize", strconv.FormatUint(uint64(c.PageSize), 10))
	if filterErr := setPageFilters(query, data); filterErr != nil {
		c.renderError(w, r, http.StatusBadRequest, query, filterErr)
		return
	}
	start := time.Now()
	page, err := c.Client.GetConferencePage(u, data)
	if err != nil {
		rest.ServerError(w, r, err)
		return
	}
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
