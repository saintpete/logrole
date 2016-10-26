package server

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/services"
)

var base, phoneTpl, copyScript, sidTpl, messageInstanceTpl, messageListTpl,
	callInstanceTpl, callListTpl, indexTpl, loginTpl, recordingTpl,
	pagingTpl, errorTpl string

// TODO move these to newServer() constructors with an error handler
var errorTemplate *template.Template
var indexTemplate *template.Template

func init() {
	base = assets.MustAssetString("templates/base.html")
	phoneTpl = assets.MustAssetString("templates/snippets/phonenumber.html")
	copyScript = assets.MustAssetString("templates/snippets/copy-phonenumber.js")
	sidTpl = assets.MustAssetString("templates/snippets/sid.html")
	messageInstanceTpl = assets.MustAssetString("templates/messages/instance.html")
	messageListTpl = assets.MustAssetString("templates/messages/list.html")
	callInstanceTpl = assets.MustAssetString("templates/calls/instance.html")
	callListTpl = assets.MustAssetString("templates/calls/list.html")
	indexTpl = assets.MustAssetString("templates/index.html")
	loginTpl = assets.MustAssetString("templates/login.html")
	recordingTpl = assets.MustAssetString("templates/calls/recordings.html")
	pagingTpl = assets.MustAssetString("templates/snippets/paging.html")
	errorTpl = assets.MustAssetString("templates/errors.html")

	errorTemplate = template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base + errorTpl))
	indexTemplate = template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base + indexTpl))
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
	Start     time.Time
	Path      string
	LoggedOut bool
	TZ        string
	LF        services.LocationFinder
	// Whatever data gets sent to the child template. Should have a Title
	// property or Title() function.
	Data interface{}
}

func tzTime(now time.Time, lf services.LocationFinder, loc string) string {
	l := lf.GetLocation(loc)
	return services.FriendlyDate(now.In(l))
}

// Render renders the given template to a bytes.Buffer. If the template renders
// successfully, we write it to the ResponseWriter, otherwise we return the
// error.
//
// data should inherit from baseData
func render(w io.Writer, r *http.Request, tpl *template.Template, name string, data *baseData) error {
	data.Start = time.Now()
	data.Path = r.URL.Path
	if data.LF != nil {
		data.TZ = data.LF.GetLocationReq(r).String()
	}
	b := templatePool.Get().(*bytes.Buffer)
	defer templatePool.Put(b)
	if err := tpl.ExecuteTemplate(b, name, data); err != nil {
		return err
	}
	_, writeErr := io.Copy(w, b)
	return writeErr
}
