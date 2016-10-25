// Most of this is copied from github.com/inconshreveable/log15/format.go, with
// some changes:
//
// - The time format prints milliseconds if a TTY, and nanoseconds if writing
// to a file.
// - The TTY format doesn't try to pad a msg.
// - The logfmt fmt doesn't try to write an empty message.

package handlers

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/inconshreveable/log15/term"
	"github.com/kevinburke/rest"
	"github.com/mattn/go-colorable"
)

const termTimeFormat = "15:04:05.000-07:00"
const timeFormat = "2006-01-02T15:04:05.000000-07:00"

const floatFormat = 'f'

// Logger is a logger configured to avoid the 40-char spacing gap between the
// message and the first key, and to write timestamps with full nanosecond
// precision.
var Logger log15.Logger

func init() {
	Logger = NewLogger()
	rest.Logger = Logger
}

// NewLogger returns a new customizable Logger, with the same initial settings
// as the package Logger.
func NewLogger() log15.Logger {
	l := log15.New()
	if term.IsTty(os.Stdout.Fd()) {
		l.SetHandler(log15.StreamHandler(colorable.NewColorableStdout(), termFormat()))
	} else {
		l.SetHandler(log15.StreamHandler(os.Stdout, logfmtFormat()))
	}
	return l
}

// LogfmtFormat prints records in logfmt format, an easy machine-parseable but human-readable
// format for key/value pairs.
//
// For more details see: http://godoc.org/github.com/kr/logfmt
//
func logfmtFormat() log15.Format {
	return log15.FormatFunc(func(r *log15.Record) []byte {
		common := []interface{}{r.KeyNames.Time, r.Time, r.KeyNames.Lvl, r.Lvl}
		if len(r.Msg) > 0 {
			common = append(common, r.KeyNames.Msg, r.Msg)
		}
		buf := &bytes.Buffer{}
		logfmt(buf, append(common, r.Ctx...), 0)
		return buf.Bytes()
	})
}

func termFormat() log15.Format {
	return log15.FormatFunc(func(r *log15.Record) []byte {
		var color = 0
		switch r.Lvl {
		case log15.LvlCrit:
			color = 35
		case log15.LvlError:
			color = 31
		case log15.LvlWarn:
			color = 33
		case log15.LvlInfo:
			color = 32
		case log15.LvlDebug:
			color = 36
		}

		b := new(bytes.Buffer)
		lvl := strings.ToUpper(r.Lvl.String())
		if color > 0 {
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s] %s ", color, lvl, r.Time.Format(termTimeFormat), r.Msg)
		} else {
			fmt.Fprintf(b, "[%s] [%s] %s ", lvl, r.Time.Format(termTimeFormat), r.Msg)
		}

		// print the keys logfmt style
		logfmt(b, r.Ctx, color)
		return b.Bytes()
	})
}

var errorKey = "HANDLER_ERROR"

func logfmt(buf *bytes.Buffer, ctx []interface{}, color int) {
	for i := 0; i < len(ctx); i += 2 {
		if i != 0 {
			buf.WriteByte(' ')
		}

		k, ok := ctx[i].(string)
		v := formatLogfmtValue(ctx[i+1])
		if !ok {
			k, v = errorKey, formatLogfmtValue(k)
		}

		// XXX: we should probably check that all of your key bytes aren't invalid
		if color > 0 {
			fmt.Fprintf(buf, "\x1b[%dm%s\x1b[0m=%s", color, k, v)
		} else {
			fmt.Fprintf(buf, "%s=%s", k, v)
		}
	}

	buf.WriteByte('\n')
}

// formatValue formats a value for serialization
func formatLogfmtValue(value interface{}) string {
	if value == nil {
		return "nil"
	}

	value = formatShared(value)
	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case float32:
		return strconv.FormatFloat(float64(v), floatFormat, 3, 64)
	case float64:
		return strconv.FormatFloat(v, floatFormat, 3, 64)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", value)
	case string:
		return escapeString(v)
	default:
		return escapeString(fmt.Sprintf("%+v", value))
	}
}

func formatShared(value interface{}) (result interface{}) {
	defer func() {
		if err := recover(); err != nil {
			if v := reflect.ValueOf(value); v.Kind() == reflect.Ptr && v.IsNil() {
				result = "nil"
			} else {
				panic(err)
			}
		}
	}()

	switch v := value.(type) {
	case time.Time:
		return v.Format(timeFormat)

	case error:
		return v.Error()

	case fmt.Stringer:
		return v.String()

	default:
		return v
	}
}

func escapeString(s string) string {
	needQuotes := false
	e := bytes.Buffer{}
	e.WriteByte('"')
	for _, r := range s {
		if r <= ' ' || r == '=' || r == '"' {
			needQuotes = true
		}

		switch r {
		case '\\', '"':
			e.WriteByte('\\')
			e.WriteByte(byte(r))
		case '\n':
			e.WriteByte('\\')
			e.WriteByte('n')
		case '\r':
			e.WriteByte('\\')
			e.WriteByte('r')
		case '\t':
			e.WriteByte('\\')
			e.WriteByte('t')
		default:
			e.WriteRune(r)
		}
	}
	e.WriteByte('"')
	start, stop := 0, e.Len()
	if !needQuotes {
		start, stop = 1, stop-1
	}
	return string(e.Bytes()[start:stop])
}
