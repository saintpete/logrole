package server

import (
	"net/http"
	"net/url"

	log "github.com/inconshreveable/log15"
	"github.com/saintpete/logrole/services"
)

type tzServer struct {
	log.Logger
	LocationFinder          services.LocationFinder
	AllowUnencryptedTraffic bool
}

func (t *tzServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO csrf
	if err := r.ParseForm(); err != nil {
		t.Warn("Error parsing form on TZ page", "err", err)
		http.Redirect(w, r, "/", 302)
		return
	}
	tz := r.PostForm.Get("tz")
	ok := t.LocationFinder.SetLocation(w, tz, t.AllowUnencryptedTraffic == false)
	if !ok {
		t.Warn("Could not set location on request", "loc", tz)
	}
	g := r.PostForm.Get("g")
	u, err := url.Parse(g)
	if err == nil {
		http.Redirect(w, r, u.Path, 302)
		return
	}
	http.Redirect(w, r, "/", 302)
}
