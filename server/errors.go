package server

import (
	"fmt"
	"html/template"
	"net/http"
	"net/mail"

	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	"github.com/saintpete/logrole/assets"
	"github.com/saintpete/logrole/services"
)

var errorTemplate *template.Template

func init() {
	base := string(assets.MustAsset("templates/base.html"))
	templates := template.Must(template.New("base").Option("missingkey=error").Funcs(funcMap).Parse(base))

	terror := template.Must(templates.Clone())
	errorTpl := string(assets.MustAsset("templates/errors.html"))
	errorTemplate = template.Must(terror.Parse(errorTpl))
}

type errorData struct {
	baseData
	Title       string
	Description string
	Mailto      *mail.Address
}

type errorServer struct {
	Mailto   *mail.Address
	Reporter services.ErrorReporter
}

func (e *errorServer) Serve401(w http.ResponseWriter, r *http.Request) {
	data := &baseData{Data: &errorData{
		Title:       "Unauthorized",
		Description: "Please enter your credentials to access this page.",
		Mailto:      e.Mailto,
	}}
	domain := rest.CtxDomain(r)
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, domain))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(401)
	if err := render(w, r, errorTemplate, "base", data); err != nil {
		handlers.Logger.Error("Error rendering error template", "err", err)
	}
}

func (e *errorServer) Serve403(w http.ResponseWriter, r *http.Request) {
	data := &baseData{Data: &errorData{
		Title:       "Forbidden",
		Description: "You don't have permission to access this page. If you think something is broken, please report a problem.",
		Mailto:      e.Mailto,
	}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(403)
	if err := render(w, r, errorTemplate, "base", data); err != nil {
		handlers.Logger.Error("Error rendering error template", "err", err)
	}
}

func (e *errorServer) Serve404(w http.ResponseWriter, r *http.Request) {
	data := &baseData{Data: &errorData{
		Title:       "Page Not Found",
		Description: "Oops, the page you're looking for does not exist. You may want to head back to the homepage. If you think something is broken, report a problem.",
		Mailto:      e.Mailto,
	}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(404)
	if err := render(w, r, errorTemplate, "base", data); err != nil {
		handlers.Logger.Info("Error rendering error template", "err", err)
	}
}

func (e *errorServer) Serve405(w http.ResponseWriter, r *http.Request) {
	data := &baseData{Data: &errorData{
		Title:       "Method not allowed",
		Description: fmt.Sprintf("You can't make a %s request to this page.", r.Method),
		Mailto:      e.Mailto,
	}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(405)
	if err := render(w, r, errorTemplate, "base", data); err != nil {
		handlers.Logger.Info("Error rendering error template", "err", err)
	}
}

func (e *errorServer) Serve500(w http.ResponseWriter, r *http.Request) {
	data := &baseData{Data: &errorData{
		Title:       "Server Error",
		Description: "We got an unexpected error when serving your request. Please refresh the page and try again. If you think something is broken, report a problem.",
		Mailto:      e.Mailto,
	}}
	err := rest.CtxErr(r)
	handlers.Logger.Error("Server error", "code", 500, "method", r.Method, "path", r.URL.Path, "err", err)
	if e.Reporter != nil {
		e.Reporter.ReportError(err, false)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(500)
	if err := render(w, r, errorTemplate, "base", data); err != nil {
		handlers.Logger.Error("Error rendering error template", "err", err)
	}
}

func registerErrorHandlers(e *errorServer) {
	rest.RegisterHandler(401, e.Serve401)
	rest.RegisterHandler(403, e.Serve403)
	rest.RegisterHandler(404, e.Serve404)
	rest.RegisterHandler(405, e.Serve405)
	rest.RegisterHandler(500, e.Serve500)
}
