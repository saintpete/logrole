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
	base := string(assets.MustAsset("templates/base.html"))
	templates := template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base))

	tlist := template.Must(templates.Clone())
	listTpl := string(assets.MustAsset("templates/messages/list.html"))
	messageListTemplate = template.Must(tlist.Parse(listTpl))

	tinstance := template.Must(templates.Clone())
	instanceTpl := string(assets.MustAsset("templates/messages/instance.html"))
	messageInstanceTemplate = template.Must(tinstance.Parse(instanceTpl))
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

func (m *messageInstanceData) Title() string {
	return "Message Details"
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := messageInstanceTemplate.ExecuteTemplate(w, "base", data); err != nil {
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
	Err      string
}

func (m *messageData) Title() string {
	return "Messages"
}

func (s *messageListServer) renderError(w http.ResponseWriter, r *http.Request, err error) {
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &messageData{Err: str, Page: new(twilio.MessagePage)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := messageListTemplate.ExecuteTemplate(w, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

func (s *messageListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := new(twilio.MessagePage)
	var err error
	next := query.Get("next")
	start := time.Now()
	if next != "" && strings.HasPrefix(next, "/"+twilio.APIVersion) {
		err = s.Client.GetNextPage(services.Unshorter(next), page)
	} else {
		// valid values: https://www.twilio.com/docs/api/rest/message#list
		data := url.Values{}
		if from := query.Get("from"); from != "" {
			fromPN, err := twilio.NewPhoneNumber(from)
			if err != nil {
				s.renderError(w, r, err)
				return
			}
			data.Set("From", string(fromPN))
		}
		if to := query.Get("to"); to != "" {
			toPN, err := twilio.NewPhoneNumber(to)
			if err != nil {
				s.renderError(w, r, err)
				return
			}
			data.Set("To", string(toPN))
		}
		// NB - we purposely don't do date validation here since we filter out
		// older messages as part of the message view.
		if startDate := query.Get("start"); startDate != "" {
			data.Set("DateSent>", startDate)
		}
		if end := query.Get("end"); end != "" {
			data.Set("DateSent<", end)
		}
		page, err = s.Client.Messages.GetPage(data)
	}
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	duration := time.Since(start)
	data := &messageData{
		Duration: duration,
		Page:     page,
		Loc:      s.Location,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := messageListTemplate.ExecuteTemplate(w, "base", data); err != nil {
		// TODO buffer here
		s.renderError(w, r, err)
	}
}
