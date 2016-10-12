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

func GetDuration(ctx context.Context) time.Duration {
	val := ctx.Value(startTime)
	if val != nil {
		return val.(time.Duration)
	}
	return time.Duration(0)
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
