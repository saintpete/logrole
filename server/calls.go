package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
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

var callListTemplate *template.Template

func init() {
	base := string(assets.MustAsset("templates/base.html"))
	templates := template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base))

	tlist := template.Must(templates.Clone())
	listTpl := string(assets.MustAsset("templates/calls/list.html"))
	callListTemplate = template.Must(tlist.Parse(listTpl))
}

type callListServer struct {
	Client         *views.Client
	Location       *time.Location
	PageSize       uint
	SecretKey      *[32]byte
	MaxResourceAge time.Duration
}

type callListData struct {
	baseData
	Page              *views.CallPage
	EncryptedNextPage string
	Loc               *time.Location
	Query             url.Values
	Err               string
	MaxResourceAge    time.Duration
}

func (c *callListData) Title() string {
	return "Calls"
}

func (c *callListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	// This is modified as we parse the query; specifically we add some values
	// if they are present in the next page URI.
	query := r.URL.Query()
	page := new(views.CallPage)
	var err error
	opaqueNext := query.Get("next")
	start := time.Now()
	if opaqueNext != "" {
		next, err := services.Unopaque(opaqueNext, c.SecretKey)
		if err != nil {
			err = errors.New("Could not decrypt `next` query parameter: " + err.Error())
			c.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			handlers.Logger.Warn("Invalid next page URI", "next", next, "opaque", opaqueNext)
			c.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, err = c.Client.GetNextCallPage(u, next)
		setNextPageValsOnQuery(next, query)
	} else {
		// valid values: https://www.twilio.com/docs/api/rest/message#list
		data := url.Values{}
		data.Set("PageSize", strconv.FormatUint(uint64(c.PageSize), 10))
		if err := setPageFilters(query, data); err != nil {
			c.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		page, err = c.Client.GetCallPage(u, data)
	}
	if err != nil {
		c.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
	data := &callListData{
		Page:           page,
		Loc:            c.Location,
		Query:          query,
		MaxResourceAge: c.MaxResourceAge,
	}
	data.Duration = time.Since(start)
	data.EncryptedNextPage, err = getEncryptedNextPage(page.NextPageURI(), c.SecretKey)
	if err != nil {
		c.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data.Start = time.Now()
	if err := render(w, callListTemplate, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

func (c *callListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &callListData{
		Err:            str,
		Query:          query,
		Page:           new(views.CallPage),
		MaxResourceAge: c.MaxResourceAge,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	data.Start = time.Now()
	if err := render(w, callListTemplate, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

// Min returns the minimum acceptable resource date, formatted for use in a
// date HTML input field.
func (c *callListData) Min() string {
	// TODO combine with the Message implementation
	return time.Now().Add(-c.MaxResourceAge).Format("2006-01-02")
}

// Max returns a the maximum acceptable resource date, formatted for use in a
// date HTML input field.
func (c *callListData) Max() string {
	return time.Now().UTC().Format("2006-01-02")
}
