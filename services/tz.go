package services

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// Turns "America/New_York" into "New York"
func FriendlyLocation(loc *time.Location) string {
	if loc == nil {
		panic("FriendlyLocation called with nil location")
	}
	s := loc.String()
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return s
	}
	return strings.Replace(parts[1], "_", " ", -1)
}

type LocationFinder interface {
	AddLocation(loc string) bool
	GetLocation(string) *time.Location
	// GetLocation gets a location preference from the user cookie, or the
	// default location if no location was found.
	GetLocationReq(*http.Request) *time.Location
	// SetLocation sets the location (string) as a cookie, and returns true if
	// it was successfully set.
	SetLocation(http.ResponseWriter, string, bool) bool
	// Locations returns all known locations
	Locations() []*time.Location
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

func (lf *locationFinder) Locations() []*time.Location {
	lf.mu.Lock()
	defer lf.mu.Unlock()
	locs := make([]*time.Location, len(lf.mp))
	i := 0
	for _, loc := range lf.mp {
		locs[i] = loc
		i++
	}
	return locs
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
