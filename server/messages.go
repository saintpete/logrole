package server

import (
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/kevinburke/rest"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/services"
)

var messageInstanceTemplate *template.Template
var messageListTemplate *template.Template
var messageInstanceRoute = regexp.MustCompile(`^/messages/(?P<sid>(MM|SM)[a-f0-9]{32})$`)

func init() {
	listIdx := string(assets.MustAsset("templates/messages/list.html"))
	messageListTemplate = template.Must(
		template.New("messages.list").Funcs(funcMap).Parse(listIdx),
	).Option("missingkey=error")

	instanceIdx := string(assets.MustAsset("templates/messages/instance.html"))
	messageInstanceTemplate = template.Must(
		template.New("messages.instance").Funcs(funcMap).Parse(instanceIdx),
	).Option("missingkey=error")
}

type messageInstanceServer struct {
	Client   *twilio.Client
	Location *time.Location
}

type messageInstanceData struct {
	Message  *twilio.Message
	Duration time.Duration
	Loc      *time.Location
	Media    *mediaResp
}

type mediaResp struct {
	Err  error
	URLs []*url.URL
}

// Just make sure we get all of the media when we make a request
var mediaUrlsFilters = url.Values{
	"PageSize": []string{"100"},
}

func (s *messageInstanceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sid := messageInstanceRoute.FindStringSubmatch(r.URL.Path)[1]
	start := time.Now()
	rch := make(chan *mediaResp, 1)
	go func(sid string) {
		urls, err := s.Client.Messages.GetMediaURLs(sid, mediaUrlsFilters)
		r := &mediaResp{
			URLs: urls,
			Err:  err,
		}
		rch <- r
		close(rch)
	}(sid)
	message, err := s.Client.Messages.Get(sid)
	if err != nil {
		rest.ServerError(w, r, err)
	}
	data := &messageInstanceData{
		Message: message,
		Loc:     s.Location,
	}
	if message.NumMedia > 0 {
		r := <-rch
		data.Media = r
	}
	data.Duration = time.Since(start)
	if err := messageInstanceTemplate.Execute(w, data); err != nil {
		rest.ServerError(w, r, err)
	}
}

type messageListServer struct {
	Client   *twilio.Client
	Location *time.Location
}

type messageData struct {
	Duration time.Duration
	Page     *twilio.MessagePage
	Loc      *time.Location
}

func (s *messageListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	start := time.Now()
	page := new(twilio.MessagePage)
	var err error
	next := query.Get("next")
	if next != "" && strings.HasPrefix(next, "/"+twilio.APIVersion) {
		err = s.Client.GetNextPage(services.Unshorter(next), page)
	} else {
		page, err = s.Client.Messages.GetPage(url.Values{})
	}
	if err != nil {
		rest.ServerError(w, r, err)
		return
	}
	duration := time.Since(start)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := messageListTemplate.Execute(w, messageData{
		Duration: duration,
		Page:     page,
		Loc:      s.Location,
	}); err != nil {
		rest.ServerError(w, r, err)
	}
}
