// +build go1.7

package handlers

import (
	"context"
	"net/http"
	"time"

	uuid "github.com/satori/go.uuid"
)

type ctxVar int

var requestID ctxVar = 0
var startTime ctxVar = 1

// SetRequestID sets the given UUID on the request context and returns the
// modified HTTP request.
func SetRequestID(r *http.Request, u uuid.UUID) *http.Request {
	r.Header.Set("X-Request-Id", u.String())
	return r.WithContext(context.WithValue(r.Context(), requestID, u))
}

// GetRequestID returns a UUID (if it exists in the context) or false if none
// could be found.
func GetRequestID(ctx context.Context) (uuid.UUID, bool) {
	val := ctx.Value(requestID)
	if val != nil {
		v, ok := val.(uuid.UUID)
		return v, ok
	}
	return uuid.UUID{}, false
}

// GetDuration returns the amount of time since the Duration handler ran, or
// 0 if no Duration was set for this context.
func GetDuration(ctx context.Context) time.Duration {
	t := GetStartTime(ctx)
	if t.IsZero() {
		return time.Duration(0)
	}
	return time.Since(t)
}

// GetStartTime returns the time the Duration handler ran.
func GetStartTime(ctx context.Context) time.Time {
	val := ctx.Value(startTime)
	if val != nil {
		t := val.(time.Time)
		return t
	}
	return time.Time{}
}

// Duration sets a the start time in the context and sets a X-Request-Duration
// header on the response, from the time this handler started executing to the
// time of the first WriteHeader() or Write() call.
func Duration(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		sw := &startWriter{
			w:           w,
			start:       start,
			wroteHeader: false,
		}
		r = r.WithContext(context.WithValue(r.Context(), startTime, start))
		h.ServeHTTP(sw, r)
	})
}

// WithTimeout sets the given timeout in the Context of every incoming request
// before passing it to the next handler.
//
// WithTimeout is only available for Go 1.7 and above.
func WithTimeout(h http.Handler, timeout time.Duration) http.Handler {
	if timeout < 0 {
		panic("invalid timeout (negative number)")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}
