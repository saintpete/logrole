package server

import (
	"html/template"
	"net/http"
	"regexp"

	log "github.com/inconshreveable/log15"
	"github.com/kevinburke/rest"
	twilio "github.com/saintpete/twilio-go"
)

type searchServer struct {
	log.Logger
}

var smsSid = regexp.MustCompile("^" + messagePattern + "$")
var callSid = regexp.MustCompile("^" + callPattern + "$")
var conferenceSid = regexp.MustCompile("^" + conferencePattern + "$")
var notificationSid = regexp.MustCompile("^" + alertPattern + "$")
var numberSid = regexp.MustCompile("^" + numberSidPattern + "$")

func (s *searchServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := query.Get("q")
	if q == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if smsSid.MatchString(q) {
		http.Redirect(w, r, "/messages/"+q, http.StatusMovedPermanently)
		return
	}
	if callSid.MatchString(q) {
		http.Redirect(w, r, "/calls/"+q, http.StatusMovedPermanently)
		return
	}
	if conferenceSid.MatchString(q) {
		http.Redirect(w, r, "/conferences/"+q, http.StatusMovedPermanently)
		return
	}
	if notificationSid.MatchString(q) {
		http.Redirect(w, r, "/alerts/"+q, http.StatusMovedPermanently)
		return
	}
	if numberSid.MatchString(q) {
		http.Redirect(w, r, "/phone-numbers/"+q, http.StatusMovedPermanently)
		return
	}
	num, err := twilio.NewPhoneNumber(q)
	if err == nil && len(num) > 3 {
		http.Redirect(w, r, "/phone-numbers/"+string(num), http.StatusFound)
	}
	s.Warn("Unknown search query", "q", q)
	http.Redirect(w, r, "/", http.StatusFound)
}

type openSearchXMLServer struct {
	PublicHost              string
	AllowUnencryptedTraffic bool
	tpl                     *template.Template
}

func newOpenSearchServer(publicHost string, allowUnencryptedTraffic bool) (*openSearchXMLServer, error) {
	openSearchTemplate, err := newTpl(template.FuncMap{}, openSearchTpl)
	if err != nil {
		return nil, err
	}
	return &openSearchXMLServer{
		PublicHost:              publicHost,
		AllowUnencryptedTraffic: allowUnencryptedTraffic,
		tpl: openSearchTemplate,
	}, nil
}

type searchData struct {
	Scheme     string
	PublicHost string
}

func (o *openSearchXMLServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if o.PublicHost == "" {
		rest.NotFound(w, r)
		return
	}
	var scheme string
	if o.AllowUnencryptedTraffic {
		scheme = "http"
	} else {
		scheme = "https"
	}
	data := &searchData{
		Scheme:     scheme,
		PublicHost: o.PublicHost,
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	if err := o.tpl.Execute(w, data); err != nil {
		rest.ServerError(w, r, err)
	}
}
