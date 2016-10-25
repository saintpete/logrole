package services

import (
	"net/http"
	"sync"
	"time"
)

type LocationFinder interface {
	AddLocation(loc string) bool
	GetLocation(string) *time.Location
	GetLocationReq(*http.Request) *time.Location
	SetLocation(http.ResponseWriter, string, bool) bool
}

// NewLocationFinder returns a new LocationFinder, where the defaultLocation
// is used for any request where we can't find the default location. If
// defaultLocation is the empty string, time.UTC will be used as the default.
// Returns an error if defaultLocation cannot be parsed by time.LoadLocation.
func NewLocationFinder(defaultLocation string) (LocationFinder, error) {
	var loc *time.Location
	if defaultLocation == "" {
		loc = time.UTC
	} else {
		var err error
		loc, err = time.LoadLocation(defaultLocation)
		if err != nil {
			return nil, err
		}
	}
	return &locationFinder{
		mp:     make(map[string]*time.Location),
		defalt: loc,
	}, nil
}

func (l *locationFinder) AddLocation(loc string) bool {
	loctn, err := time.LoadLocation(loc)
	if err != nil {
		return false
	}
	l.mu.Lock()
	l.mp[loc] = loctn
	l.mu.Unlock()
	return true
}

func (lf *locationFinder) key() string {
	return "tz"
}

func (lf *locationFinder) GetLocation(loc string) *time.Location {
	lf.mu.Lock()
	l, ok := lf.mp[loc]
	lf.mu.Unlock()
	if !ok {
		return lf.defalt
	}
	return l
}

func (lf *locationFinder) GetLocationReq(r *http.Request) *time.Location {
	cookie, err := r.Cookie(lf.key())
	if err != nil {
		return lf.defalt
	}
	lf.mu.Lock()
	defer lf.mu.Unlock()
	loc, ok := lf.mp[cookie.Value]
	if !ok {
		return lf.defalt
	}
	return loc
}

func (lf *locationFinder) SetLocation(w http.ResponseWriter, loc string, secure bool) bool {
	lf.mu.Lock()
	defer lf.mu.Unlock()
	if _, ok := lf.mp[loc]; !ok {
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     lf.key(),
		Value:    loc,
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		MaxAge:   60 * 60 * 24 * 365,
	})
	return true
}

type locationFinder struct {
	mp     map[string]*time.Location
	mu     sync.Mutex
	defalt *time.Location
}
