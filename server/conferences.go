package server

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/kevinburke/rest"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/views"
)

type conferenceListServer struct {
	log.Logger
	Client   views.Client
	PageSize uint
	tpl      *template.Template
}

type conferenceListData struct {
}

func (d *conferenceListData) Title() string {
	return "Conferences"
}

func newConferenceListServer(l log.Logger, vc views.Client, pageSize uint) (*conferenceListServer, error) {
	tpl, err := newTpl(template.FuncMap{
		"is_our_pn": vc.IsTwilioNumber,
	}, base+conferenceListTpl)
	if err != nil {
		return nil, err
	}
	return &conferenceListServer{
		Client:   vc,
		PageSize: pageSize,
		tpl:      tpl,
	}, nil
}

func (c *conferenceListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {

}

func (c *conferenceListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewCalls() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
		return
	}
	data := url.Values{}
	data.Set("PageSize", strconv.FormatUint(uint64(c.PageSize), 10))
	start := time.Now()
	page, err := c.Client.GetConferencePage(u, data)
	if err != nil {
		c.renderError(w, r, 500, r.URL.Query(), err)
		return
	}
	fmt.Println("page", page)
	if err := render(w, r, c.tpl, "base", &baseData{
		Duration: time.Since(start),
		Data:     &conferenceListData{},
	}); err != nil {
		rest.ServerError(w, r, err)
	}
}
