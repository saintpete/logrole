package server

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kevinburke/handlers"
	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/services"
)

var base, phoneTpl, copyScript, sidTpl, messageInstanceTpl, messageListTpl,
	callInstanceTpl, callListTpl, conferenceListTpl, conferenceInstanceTpl,
	alertListTpl,
	indexTpl, loginTpl, recordingTpl, pagingTpl, openSearchTpl,
	openSourceTpl, errorTpl string

// TODO move these to newServer() constructors with an error handler
var errorTemplate *template.Template
var loginTemplate *template.Template
var openSearchTemplate *template.Template
var openSourceTemplate *template.Template

func init() {
	base = assets.MustAssetString("templates/base.html")
	phoneTpl = assets.MustAssetString("templates/snippets/phonenumber.html")
	copyScript = assets.MustAssetString("templates/snippets/copy-phonenumber.js")
	sidTpl = assets.MustAssetString("templates/snippets/sid.html")
	messageInstanceTpl = assets.MustAssetString("templates/messages/instance.html")
	messageListTpl = assets.MustAssetString("templates/messages/list.html")
	callInstanceTpl = assets.MustAssetString("templates/calls/instance.html")
	callListTpl = assets.MustAssetString("templates/calls/list.html")
	conferenceInstanceTpl = assets.MustAssetString("templates/conferences/instance.html")
	conferenceListTpl = assets.MustAssetString("templates/conferences/list.html")
	alertListTpl = assets.MustAssetString("templates/alerts/list.html")
	indexTpl = assets.MustAssetString("templates/index.html")
	loginTpl = assets.MustAssetString("templates/login.html")
	recordingTpl = assets.MustAssetString("templates/calls/recordings.html")
	pagingTpl = assets.MustAssetString("templates/snippets/paging.html")
	openSearchTpl = assets.MustAssetString("templates/opensearch.xml")
	errorTpl = assets.MustAssetString("templates/errors.html")
	openSourceTpl = assets.MustAssetString("templates/opensource.html")

	errorTemplate = template.Must(newTpl(template.FuncMap{}, base+errorTpl))
	loginTemplate = template.Must(newTpl(template.FuncMap{}, base+loginTpl))
	openSearchTemplate = template.Must(newTpl(template.FuncMap{}, openSearchTpl))
}

// newTpl creates a new Template with the given base and common set of
// functions.
func newTpl(mp template.FuncMap, tpls string) (*template.Template, error) {
	t := template.New("base").Option("missingkey=error").Funcs(funcMap)
	t = t.Funcs(mp)
	return t.Parse(tpls)
}

// Shown in the copyright notice
var year = time.Now().UTC().Year()

// renderTime returns the amount of time since we began rendering this
// template; it's designed to approximate the amount of time spent in the
// render phase on the server.
func renderTime(start time.Time) string {
	return services.Duration(time.Since(start))
}

var funcMap = template.FuncMap{
	"year":          func() int { return year },
	"friendly_date": services.FriendlyDate,
	"friendly_loc":  services.FriendlyLocation,
	"duration":      services.Duration,
	"render":        renderTime,
	"truncate_sid":  services.TruncateSid,
	"prefix_strip":  stripPrefix("+1 "),
	"tztime":        tzTime,
}

// stripPrefix strips the prefix from a phone number - in this case we strip
// the US prefix "+1 " from numbers. We could make this configurable.
func stripPrefix(pfx string) func(string) string {
	return func(val string) string {
		return strings.TrimPrefix(val, pfx)
	}
}

var templatePool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

type baseData struct {
	Duration  time.Duration
	ReqStart  time.Time
	CachedAt  time.Time
	Start     time.Time
	Path      string
	LoggedOut bool
	TZ        string
	LF        services.LocationFinder
	// Whatever data gets sent to the child template. Should have a Title
	// property or Title() function.
	Data interface{}
}

func (bd *baseData) Version() string {
	return Version
}

func tzTime(now time.Time, lf services.LocationFinder, loc string) string {
	l := lf.GetLocation(loc)
	return services.FriendlyDate(now.In(l))
}

func min(age time.Duration) string {
	return time.Now().Add(-age).Truncate(time.Hour).Format(HTML5DatetimeLocalFormat)
}

func minLoc(age time.Duration, l *time.Location) string {
	return time.Now().In(l).Add(-age).Truncate(time.Hour).Format(HTML5DatetimeLocalFormat)
}

// minFunc returns a function that, when called, returns the minimum acceptable
// age for a resource, formatted using the HTML5 Datetime format.
func minFunc(age time.Duration) func(*time.Location) string {
	return func(loc *time.Location) string {
		return minLoc(age, loc)
	}
}

func maxLoc(l *time.Location) string {
	return time.Now().In(l).Add(1 * time.Hour).Truncate(time.Hour).Format(HTML5DatetimeLocalFormat)
}

// Render renders the given template to a bytes.Buffer. If the template renders
// successfully, we write it to the ResponseWriter, otherwise we return the
// error.
//
// data should inherit from baseData
func render(w io.Writer, r *http.Request, tpl *template.Template, name string, data *baseData) error {
	data.Start = time.Now()
	data.Path = r.URL.Path
	data.ReqStart = handlers.GetStartTime(r.Context())
	if data.LF != nil {
		data.TZ = data.LF.GetLocationReq(r).String()
	}
	b := templatePool.Get().(*bytes.Buffer)
	defer func(buf *bytes.Buffer) {
		buf.Reset()
		templatePool.Put(buf)
	}(b)
	if err := tpl.ExecuteTemplate(b, name, data); err != nil {
		return err
	}
	if b.Len() == 0 {
		return errors.New("Rendered a zero length template")
	}
	_, writeErr := io.Copy(w, b)
	return writeErr
}
