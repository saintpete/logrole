// Package handlers implements a number of useful HTTP middlewares.
//
// The general format of the middlewares in this package is to wrap an existing
// http.Handler in another one. So if you have a ServeMux, you can simply do:
//
//     mux := http.NewServeMux()
//     h := handlers.Log(handlers.Debug(mux))
//     http.ListenAndServe(":5050", h)
//
// And wrap as many handlers as you'd like using that idiom.
package handlers

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kevinburke/rest"
	"github.com/satori/go.uuid"
)

const Version = "0.18"

// All wraps h with every handler in this file.
func All(h http.Handler, serverName string) http.Handler {
	return Duration(Log(Debug(UUID(JSON(Server(h, serverName))))))
}

// JSON sets the Content-Type to application/json; charset=utf-8
func JSON(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		h.ServeHTTP(w, r)
	})
}

// startWriter is used by Duration in ctx.go/noctx.go
type startWriter struct {
	w           http.ResponseWriter
	start       time.Time
	wroteHeader bool
}

func (s *startWriter) duration() string {
	d := (time.Since(s.start) / (100 * time.Microsecond)) * (100 * time.Microsecond)
	return d.String()
}

func (s *startWriter) WriteHeader(code int) {
	if s.wroteHeader == false {
		s.w.Header().Set("X-Request-Duration", s.duration())
		s.wroteHeader = true
	}
	s.w.WriteHeader(code)
}

func (s *startWriter) Write(b []byte) (int, error) {
	// Some chunked encoding transfers won't ever call WriteHeader(), so set
	// the header here.
	if s.wroteHeader == false {
		s.w.Header().Set("X-Request-Duration", s.duration())
		s.wroteHeader = true
	}
	return s.w.Write(b)
}

func (s *startWriter) Header() http.Header {
	return s.w.Header()
}

type serverWriter struct {
	w           http.ResponseWriter
	name        string
	wroteHeader bool
}

func (s *serverWriter) WriteHeader(code int) {
	if s.wroteHeader == false {
		s.w.Header().Set("Server", s.name)
		s.wroteHeader = true
	}
	s.w.WriteHeader(code)
}

func (s *serverWriter) Write(b []byte) (int, error) {
	if s.wroteHeader == false {
		s.w.Header().Set("Server", s.name)
		s.wroteHeader = true
	}
	return s.w.Write(b)
}

func (s *serverWriter) Header() http.Header {
	return s.w.Header()
}

// TrailingSlashRedirect redirects any path that ends with a "/" - say,
// "/messages/" - to the stripped version, say "/messages".
func TrailingSlashRedirect(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 1 && strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path[:len(r.URL.Path)-1], http.StatusMovedPermanently)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// Server attaches a Server header to the response.
func Server(h http.Handler, serverName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &serverWriter{
			w:           w,
			name:        serverName,
			wroteHeader: false,
		}
		h.ServeHTTP(sw, r)
	})
}

// UUID attaches a X-Request-Id header to the request, and sets one on the
// request context, unless one already exists.
func UUID(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			r = SetRequestID(r, uuid.NewV4())
		}
		h.ServeHTTP(w, r)
	})
}

// BasicAuth protects all requests to the given handler, unless the request has
// basic auth with a username and password in the users map.
func BasicAuth(h http.Handler, realm string, users map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			rest.Unauthorized(w, r, realm)
			return
		}

		serverPass, ok := users[user]
		if !ok {
			if user == "" {
				rest.Unauthorized(w, r, realm)
			} else {
				rest.Forbidden(w, r, &rest.Error{
					Title: "Username or password are invalid. Please double check your credentials",
					ID:    "forbidden",
				})
			}
			return
		}
		if subtle.ConstantTimeCompare([]byte(pass), []byte(serverPass)) != 1 {
			rest.Forbidden(w, r, &rest.Error{
				Title:    fmt.Sprintf("Incorrect password for user %s", user),
				ID:       "incorrect_password",
				Instance: r.URL.Path,
			})
			return
		}
		h.ServeHTTP(w, r)
	})
}

// Debug prints debugging information about the request to stdout if the
// DEBUG_HTTP_TRAFFIC environment variable is set to true.
func Debug(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("DEBUG_HTTP_TRAFFIC") == "true" {
			// You need to write the entire thing in one Write, otherwise the
			// output will be jumbled with other requests.
			b := new(bytes.Buffer)
			bits, err := httputil.DumpRequest(r, true)
			if err != nil {
				_, _ = b.WriteString(err.Error())
			} else {
				_, _ = b.Write(bits)
			}
			res := httptest.NewRecorder()
			h.ServeHTTP(res, r)

			_, _ = b.WriteString(fmt.Sprintf("HTTP/1.1 %d\r\n", res.Code))
			_ = res.HeaderMap.Write(b)
			for k, v := range res.HeaderMap {
				w.Header()[k] = v
			}
			w.WriteHeader(res.Code)
			_, _ = b.WriteString("\r\n")
			writer := io.MultiWriter(w, b)
			_, _ = res.Body.WriteTo(writer)
			_, _ = b.WriteTo(os.Stderr)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

// responseLogger is wrapper of http.ResponseWriter that keeps track of its HTTP
// status code and body size
type responseLogger struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (l *responseLogger) Header() http.Header {
	return l.w.Header()
}

func (l *responseLogger) Write(b []byte) (int, error) {
	if l.status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		l.status = http.StatusOK
	}
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

func (l *responseLogger) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

func (l *responseLogger) Status() int {
	return l.status
}

func (l *responseLogger) Size() int {
	return l.size
}

func (l *responseLogger) Flush() {
	f, ok := l.w.(http.Flusher)
	if ok {
		f.Flush()
	}
}

type hijackLogger struct {
	responseLogger
}

type hijackCloseNotifier struct {
	loggingResponseWriter
	http.Hijacker
	http.CloseNotifier
}

type closeNotifyWriter struct {
	loggingResponseWriter
	http.CloseNotifier
}

func makeLogger(w http.ResponseWriter) loggingResponseWriter {
	var logger loggingResponseWriter = &responseLogger{w: w}
	if _, ok := w.(http.Hijacker); ok {
		logger = &hijackLogger{responseLogger{w: w}}
	}
	h, ok1 := logger.(http.Hijacker)
	c, ok2 := w.(http.CloseNotifier)
	if ok1 && ok2 {
		return hijackCloseNotifier{logger, h, c}
	}
	if ok2 {
		return &closeNotifyWriter{logger, c}
	}
	return logger
}

type loggingResponseWriter interface {
	http.ResponseWriter
	http.Flusher
	Status() int
	Size() int
}

type logHandler struct {
	h http.Handler
}

func (l logHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	logger := makeLogger(w)
	u := *r.URL
	l.h.ServeHTTP(logger, r)
	writeLog(r, u, t, logger.Status(), logger.Size())
}

func getRemoteIP(r *http.Request) string {
	fwd := r.Header.Get("X-Forwarded-For")
	if fwd == "" {
		return r.RemoteAddr
	}
	return strings.Split(fwd, ",")[0]
}

// Return the time since the given time, in ms.
func timeSinceMs(t time.Time) int64 {
	// Add 500 microseconds so we round up or down to the nearest MS.
	ns := time.Since(t).Nanoseconds() + 500*int64(time.Microsecond)
	return ns / int64(time.Millisecond)
}

func writeLog(r *http.Request, u url.URL, t time.Time, status int, size int) {
	user, _, _ := r.BasicAuth()
	args := []interface{}{
		"user", user,
		"method", r.Method,
		"path", r.URL.RequestURI(),
		"time", strconv.FormatInt(timeSinceMs(t), 10),
		"bytes", strconv.Itoa(size),
		"status", strconv.Itoa(status),
		"remote_addr", getRemoteIP(r),
		"host", r.Host,
		"user_agent", r.UserAgent(),
	}
	if user != "" {
		args = append(args, "user", user)
	}
	if r.Header.Get("X-Request-Id") != "" {
		args = append(args, "request_id", r.Header.Get("X-Request-Id"))
	}
	Logger.Info("", args...)
}

// Log serves the http request and writes information about the
// request/response to w. Any errors writing to w are ignored.
func Log(h http.Handler) http.Handler {
	return &logHandler{h}
}
