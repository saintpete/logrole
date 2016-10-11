// +build go1.7

package handlers

import (
	"context"
	"net/http"

	uuid "github.com/satori/go.uuid"
)

type RequestID int

var requestID RequestID = 0

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
