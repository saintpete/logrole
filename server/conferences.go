package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/kevinburke/rest"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/views"
)

type conferenceListServer struct {
	log.Logger
	Client         views.Client
	PageSize       uint
	MaxResourceAge time.Duration
	tpl            *template.Template
}

type conferenceListData struct {
	Query url.Values
	Page  *views.ConferencePage
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

func newConferenceListServer(l log.Logger, vc views.Client, pageSize uint, maxResourceAge time.Duration) (*conferenceListServer, error) {
	s := &conferenceListServer{
		Client:         vc,
		PageSize:       pageSize,
		MaxResourceAge: maxResourceAge,
	}
	tpl, err := newTpl(template.FuncMap{
		"is_our_pn": vc.IsTwilioNumber,
		"min":       minFunc(s.MaxResourceAge),
		"max":       max,
	}, base+conferenceListTpl)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
	return s, nil
}

func (c *conferenceListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {

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
	data := url.Values{}
	data.Set("PageSize", strconv.FormatUint(uint64(c.PageSize), 10))
	start := time.Now()
	page, err := c.Client.GetConferencePage(u, data)
	if err != nil {
		rest.ServerError(w, r, err)
		return
	}
	if err := render(w, r, c.tpl, "base", &baseData{
		Duration: time.Since(start),
		Data: &conferenceListData{
			Query: r.URL.Query(),
			Page:  page,
		},
	}); err != nil {
		rest.ServerError(w, r, err)
	}
}
