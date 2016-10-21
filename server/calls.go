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

	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
)

var callListTemplate *template.Template
var callInstanceTemplate *template.Template

const callPattern = `(?P<sid>CA[a-f0-9]{32})`

var callInstanceRoute = regexp.MustCompile("^/calls/" + callPattern + "$")

func init() {
	base := string(assets.MustAsset("templates/base.html"))
	templates := template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base))

	tlist := template.Must(templates.Clone())
	listTpl := string(assets.MustAsset("templates/calls/list.html"))
	callListTemplate = template.Must(tlist.Parse(listTpl))

	tinstance := template.Must(templates.Clone())
	instanceTpl := string(assets.MustAsset("templates/calls/instance.html"))
	callInstanceTemplate = template.Must(tinstance.Parse(instanceTpl))
	// this is ugly, would be nice to chain Parse's somehow.
	recordingTpl := string(assets.MustAsset("templates/calls/recordings.html"))
	callInstanceTemplate = template.Must(callInstanceTemplate.Parse(recordingTpl))
}

type callListServer struct {
	Client         *views.Client
	Location       *time.Location
	PageSize       uint
	SecretKey      *[32]byte
	MaxResourceAge time.Duration
}

type callInstanceServer struct {
	Client   *views.Client
	Location *time.Location
}

type callInstanceData struct {
	baseData
	Call       *views.Call
	Loc        *time.Location
	Recordings *recordingResp
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

func (c *callListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	data.EncryptedNextPage = getEncryptedNextPage(page.NextPageURI(), c.SecretKey)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data.Start = time.Now()
	data.Path = r.URL.Path
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
	data.Path = r.URL.Path
	if err := render(w, callListTemplate, "base", data); err != nil {
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

func (c *callInstanceServer) fetchRecordings(sid string, u *config.User, rch chan<- *recordingResp) {
	defer close(rch)
	if u.CanViewNumRecordings() == false {
		rch <- &recordingResp{
			Err:                  config.PermissionDenied,
			CanViewNumRecordings: false,
		}
		return
	}
	rp, err := c.Client.GetCallRecordings(u, sid, nil)
	if err != nil {
		rch <- &recordingResp{Err: err}
		return
	}
	rs := rp.Recordings()
	uri := rp.NextPageURI()
	for uri.Valid {
		rp, err := c.Client.GetNextRecordingPage(u, uri.String)
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
	sid := callInstanceRoute.FindStringSubmatch(r.URL.Path)[1]
	start := time.Now()
	rch := make(chan *recordingResp, 1)
	go c.fetchRecordings(sid, u, rch)
	call, err := c.Client.GetCall(u, sid)
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
	data := &callInstanceData{
		Call: call,
		Loc:  c.Location,
	}
	data.Duration = time.Since(start)
	data.Start = time.Now()
	if u.CanViewNumRecordings() {
		r := <-rch
		data.Recordings = r
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render(w, callInstanceTemplate, "base", data); err != nil {
		rest.ServerError(w, r, err)
		return
	}
}
