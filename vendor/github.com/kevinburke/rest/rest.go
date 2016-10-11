// Package rest implements responses and a HTTP client for API consumption.
package rest

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/inconshreveable/log15"
)

// Logger logs information about incoming requests.
var Logger log.Logger = log.New()

// Error implements the HTTP Problem spec laid out here:
// https://tools.ietf.org/html/draft-ietf-appsawg-http-problem-03
type Error struct {
	// The main error message. Should be short enough to fit in a phone's
	// alert box. Do not end this message with a period.
	Title string `json:"title"`

	// Id of this error message ("forbidden", "invalid_parameter", etc)
	ID string `json:"id"`

	// More information about what went wrong.
	Detail string `json:"detail,omitempty"`

	// Path to the object that's in error.
	Instance string `json:"instance,omitempty"`

	// Link to more information (Zendesk, API docs, etc)
	Type       string `json:"type,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
}

func (e *Error) Error() string {
	return e.Title
}

func (e *Error) String() string {
	if e.Detail != "" {
		return fmt.Sprintf("rest: %s. %s", e.Title, e.Detail)
	} else {
		return fmt.Sprintf("rest: %s", e.Title)
	}
}

var serverError = Error{
	StatusCode: http.StatusInternalServerError,
	ID:         "server_error",
	Title:      "Unexpected server error. Please try again",
}

// ServerError logs the error to the Logger, and then responds to the request
// with a generic 500 server error message. ServerError panics if err is nil.
func ServerError(w http.ResponseWriter, r *http.Request, err error) error {
	if err == nil {
		panic("rest: no error to log")
	}
	Logger.Info(fmt.Sprintf("500: %s %s: %s", r.Method, r.URL.Path, err))
	w.WriteHeader(http.StatusInternalServerError)
	return json.NewEncoder(w).Encode(serverError)
}

var notFound = Error{
	Title:      "Resource not found",
	ID:         "not_found",
	StatusCode: http.StatusNotFound,
}

// NotFound returns a 404 Not Found error to the client.
func NotFound(w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(http.StatusNotFound)
	nf := notFound
	nf.Instance = r.URL.Path
	return json.NewEncoder(w).Encode(nf)
}

// BadRequest logs a 400 error and then returns a 400 response to the client.
func BadRequest(w http.ResponseWriter, r *http.Request, err *Error) error {
	if err == nil {
		panic("rest: no error to write")
	}
	if err.StatusCode == 0 {
		err.StatusCode = http.StatusBadRequest
	}
	Logger.Info(fmt.Sprintf("400: %s", err.Error()), "method", r.Method, "path", r.URL.Path)
	w.WriteHeader(http.StatusBadRequest)
	return json.NewEncoder(w).Encode(err)
}

var notAllowed = Error{
	Title:      "Method not allowed",
	ID:         "method_not_allowed",
	StatusCode: http.StatusMethodNotAllowed,
}

var authenticate = Error{
	Title:      "Unauthorized. Please include your API credentials",
	ID:         "unauthorized",
	StatusCode: http.StatusUnauthorized,
}

// NotAllowed returns a generic HTTP 405 Not Allowed status and response body
// to the client.
func NotAllowed(w http.ResponseWriter, r *http.Request) error {
	e := notAllowed
	e.Instance = r.URL.Path
	w.WriteHeader(http.StatusMethodNotAllowed)
	return json.NewEncoder(w).Encode(e)
}

// Forbidden returns a 403 Forbidden status code to the client, with the given
// Error object in the response body.
func Forbidden(w http.ResponseWriter, r *http.Request, err *Error) error {
	w.WriteHeader(http.StatusForbidden)
	return json.NewEncoder(w).Encode(err)
}

// NoContent returns a 204 No Content message.
func NoContent(w http.ResponseWriter) {
	w.Header().Del("Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func Unauthorized(w http.ResponseWriter, r *http.Request, domain string) error {
	err := authenticate
	err.Instance = r.URL.Path
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, domain))
	w.WriteHeader(http.StatusUnauthorized)
	return json.NewEncoder(w).Encode(err)
}
