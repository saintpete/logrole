package server

import (
	"html/template"
	"net/http"
	"regexp"

	"github.com/kevinburke/rest"
)

var openSearchTemplate *template.Template

func init() {
	openSearchTemplate = template.Must(template.New("opensearch.xml").Option("missingkey=error").Parse(openSearchTpl))
}

type searchServer struct{}

var smsSid = regexp.MustCompile(messagePattern)

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
	http.Redirect(w, r, "/", http.StatusFound)
}

type openSearchXMLServer struct {
	PublicHost              string
	AllowUnencryptedTraffic bool
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
	if err := openSearchTemplate.Execute(w, data); err != nil {
		rest.ServerError(w, r, err)
	}
}

// Described here: http://stackoverflow.com/a/7630169/329700
var openSearchTpl = `
<OpenSearchDescription xmlns="http://a9.com/-/spec/opensearch/1.1/" xmlns:moz="http://www.mozilla.org/2006/browser/search/">
<ShortName>Logrole</ShortName>
<Description>
    Quick jump to a given resource
</Description>
<InputEncoding>UTF-8</InputEncoding>
<Url type="text/html" method="get" template="{{ .Scheme }}://{{ .PublicHost }}/search?q={searchTerms}"/>
</OpenSearchDescription>
`
