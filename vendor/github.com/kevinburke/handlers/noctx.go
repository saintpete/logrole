// +build !go1.7

package handlers

import (
	"net/http"

	"github.com/satori/go.uuid"
)

// SetRequestID sets the given UUID on the request and returns the modified
// HTTP request.
func SetRequestID(r *http.Request, u uuid.UUID) *http.Request {
	r.Header.Set("X-Request-Id", u.String())
	return r
}

// GetRequestID returns a UUID (if it exists on r) or false if none could
// be found.
func GetRequestID(r *http.Request) (uuid.UUID, bool) {
	rid := r.Header.Get("X-Request-Id")
	if rid != "" {
		u, err := uuid.FromString(rid)
		if err == nil {
			return u, true
		}
	}
	return uuid.UUID{}, false
}
