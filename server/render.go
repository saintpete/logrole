package server

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/saintpete/logrole/services"
)

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
