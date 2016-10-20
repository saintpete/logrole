package services

import (
	"net/http"
	"sync"

	raven "github.com/getsentry/raven-go"
)

// An ErrorReporter reports errors to a third party service. ErrorReporter
// instances should be thread safe.
type ErrorReporter interface {
	Configure(token string)
	// ReportError reports an error. If ReportError captures a stacktrace, be
	// sure to call it in the same goroutine as the context you are hoping
	// to capture. Set block to true to wait for the remote service call to
	// complete before returning.
	ReportError(err error, block bool)
	// ReportPanic returns a http.Handler that monitors the inner handler for
	// panics, and reports them to the remote service.
	ReportPanics(http.Handler) http.Handler
}

var reporters = map[string]ErrorReporter{}
var reporterMu sync.Mutex

func init() {
	RegisterReporter("sentry", new(SentryErrorReporter))
	RegisterReporter("noop", new(NoopErrorReporter))
}

// RegisterReporter allows the ErrorReporter with the given name to be used.
// Use this to register a custom ErrorReporter for your project.
//
// Call RegisterReporter(name, nil) to delete a Reporter.
func RegisterReporter(name string, r ErrorReporter) {
	reporterMu.Lock()
	defer reporterMu.Unlock()
	if r == nil {
		delete(reporters, name)
		return
	}
	reporters[name] = r
}

func IsRegistered(name string) bool {
	reporterMu.Lock()
	defer reporterMu.Unlock()
	_, ok := reporters[name]
	return ok
}

// GetReporter gets the reporter for the given name and token. If the name is
// unknown, a NoopErrorReporter is returned.
func GetReporter(name, token string) ErrorReporter {
	reporterMu.Lock()
	defer reporterMu.Unlock()
	r, ok := reporters[name]
	if !ok {
		r = new(NoopErrorReporter)
	}
	r.Configure(token)
	return r
}

type SentryErrorReporter struct{}

func (s *SentryErrorReporter) Configure(token string) {
	raven.SetDSN(token)
}

func (s *SentryErrorReporter) ReportError(err error, block bool) {
	if block {
		raven.CaptureErrorAndWait(err, nil)
	} else {
		raven.CaptureError(err, nil)
	}
}

func (s *SentryErrorReporter) ReportPanics(h http.Handler) http.Handler {
	// yuck, https://github.com/getsentry/raven-go/issues/78
	return http.HandlerFunc(raven.RecoveryHandler(h.ServeHTTP))
}

// A NoopErrorReporter silently swallows all errors.
type NoopErrorReporter struct{}

func (n *NoopErrorReporter) Configure(_ string)          {}
func (n *NoopErrorReporter) ReportError(_ error, _ bool) {}
func (n *NoopErrorReporter) ReportPanics(h http.Handler) http.Handler {
	return h
}
