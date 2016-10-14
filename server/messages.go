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
	"github.com/saintpete/logrole/services"
)

var messageInstanceTemplate *template.Template
var messageListTemplate *template.Template
var messagePattern = `(?P<sid>(MM|SM)[a-f0-9]{32})`
var messageInstanceRoute = regexp.MustCompile("^/messages/" + messagePattern + "$")

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
	Client    *twilio.Client
	Location  *time.Location
	PageSize  uint
	SecretKey *[32]byte
}

type messageData struct {
	Duration          time.Duration
	Page              *twilio.MessagePage
	EncryptedNextPage string
	Loc               *time.Location
	Query             url.Values
	Err               string
}

func (m *messageData) Title() string {
	return "Messages"
}

func (s *messageListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &messageData{Err: str, Query: query, Page: new(twilio.MessagePage)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := messageListTemplate.ExecuteTemplate(w, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

// setNextPageValsOnQuery takes query values that have been sent to the Twilio
// API, and sets them on the provided query object. We use this to populate the
// search fields on the search page.
func setNextPageValsOnQuery(nextpageuri string, query url.Values) {
	u, err := url.Parse(nextpageuri)
	if err != nil {
		return
	}
	nq := u.Query()
	if start := nq.Get("DateSent>"); start != "" {
		query.Set("start", start)
	}
	if end := nq.Get("DateSent<"); end != "" {
		query.Set("end", end)
	}
	if from := nq.Get("From"); from != "" {
		query.Set("from", from)
	}
	if to := nq.Get("To"); to != "" {
		query.Set("to", to)
	}
}

// Reverse of the function above, with validation
func setPageFilters(query url.Values, pageFilters url.Values) error {
	if from := query.Get("from"); from != "" {
		fromPN, err := twilio.NewPhoneNumber(from)
		if err != nil {
			query.Del("from")
			return err
		}
		pageFilters.Set("From", string(fromPN))
	}
	if to := query.Get("to"); to != "" {
		toPN, err := twilio.NewPhoneNumber(to)
		if err != nil {
			query.Del("to")
			return err
		}
		pageFilters.Set("To", string(toPN))
	}
	// NB - we purposely don't do date validation here since we filter out
	// older messages as part of the message view.
	if startDate := query.Get("start"); startDate != "" {
		pageFilters.Set("DateSent>", startDate)
	}
	if end := query.Get("end"); end != "" {
		pageFilters.Set("DateSent<", end)
	}
	return nil
}

func (s *messageListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// This is modified as we parse the query; specifically we add some values
	// if they are present in the next page URI.
	query := r.URL.Query()
	page := new(twilio.MessagePage)
	var err error
	opaqueNext := query.Get("next")
	start := time.Now()
	if opaqueNext != "" {
		next, err := services.Unopaque(opaqueNext, s.SecretKey)
		if err != nil {
			err = errors.New("Could not decrypt `next` query parameter: " + err.Error())
			s.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			handlers.Logger.Warn("Invalid next page URI", "next", next, "opaque", opaqueNext)
			s.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		err = s.Client.GetNextPage(next, page)
		setNextPageValsOnQuery(next, query)
	} else {
		// valid values: https://www.twilio.com/docs/api/rest/message#list
		data := url.Values{}
		data.Set("PageSize", strconv.FormatUint(uint64(s.PageSize), 10))
		if err := setPageFilters(query, data); err != nil {
			s.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		page, err = s.Client.Messages.GetPage(data)
	}
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
	duration := time.Since(start)
	data := &messageData{
		Duration: duration,
		Page:     page,
		Loc:      s.Location,
		Query:    query,
	}
	if page.NextPageURI.Valid {
		next, err := services.Opaque(page.NextPageURI.String, s.SecretKey)
		if err != nil {
			s.renderError(w, r, http.StatusInternalServerError, query, err)
			return
		}
		data.EncryptedNextPage = next
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := messageListTemplate.ExecuteTemplate(w, "base", data); err != nil {
		// TODO buffer here
		s.renderError(w, r, http.StatusInternalServerError, query, err)
		return
	}
}
