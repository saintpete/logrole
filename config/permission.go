// Package config maintains information about permissions.
//
// The format and API's in this package will probably change over time.
package config

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

type User struct {
	canViewNumMedia      bool
	canViewMessages      bool
	canViewMessageFrom   bool
	canViewMessageTo     bool
	canViewMessageBody   bool
	canViewMedia         bool
	canViewCalls         bool
	canViewCallFrom      bool
	canViewCallTo        bool
	canViewNumRecordings bool
	canPlayRecordings    bool
}

// UserSettings are used to define which permissions a User has.
type UserSettings struct {
	CanViewNumMedia    bool
	CanViewMessages    bool
	CanViewMessageFrom bool
	CanViewMessageTo   bool
	CanViewMessageBody bool
	CanViewMedia       bool
	CanViewCalls       bool
	CanViewCallFrom    bool
	CanViewCallTo      bool
	// Can the user see whether a call has recordings attached?
	CanViewNumRecordings bool
	// Can the user listen to recordings?
	CanPlayRecordings bool
}

// AllUserSettings returns a UserSettings value with the widest possible set of
// permissions.
func AllUserSettings() *UserSettings {
	return &UserSettings{
		CanViewNumMedia:      true,
		CanViewMessages:      true,
		CanViewMessageFrom:   true,
		CanViewMessageTo:     true,
		CanViewMessageBody:   true,
		CanViewMedia:         true,
		CanViewCalls:         true,
		CanViewCallFrom:      true,
		CanViewCallTo:        true,
		CanViewNumRecordings: true,
		CanPlayRecordings:    true,
	}
}

// NewUser creates a new User with the given settings.
func NewUser(us *UserSettings) *User {
	if us == nil {
		us = &UserSettings{}
	}
	return &User{
		canViewNumMedia:      us.CanViewNumMedia,
		canViewMessages:      us.CanViewMessages,
		canViewMessageFrom:   us.CanViewMessageFrom,
		canViewMessageTo:     us.CanViewMessageTo,
		canViewMessageBody:   us.CanViewMessageBody,
		canViewMedia:         us.CanViewMedia,
		canViewCalls:         us.CanViewCalls,
		canViewCallFrom:      us.CanViewCallFrom,
		canViewCallTo:        us.CanViewCallTo,
		canViewNumRecordings: us.CanViewNumRecordings,
		canPlayRecordings:    us.CanPlayRecordings,
	}
}

func (u *User) CanViewNumMedia() bool {
	return u.CanViewMessages() && u.canViewNumMedia
}

func (u *User) CanViewMessages() bool {
	return u.canViewMessages
}

func (u *User) CanViewMessageFrom() bool {
	return u.CanViewMessages() && u.canViewMessageFrom
}

func (u *User) CanViewMessageTo() bool {
	return u.CanViewMessages() && u.canViewMessageTo
}

func (u *User) CanViewMessageBody() bool {
	return u.CanViewMessages() && u.canViewMessageBody
}

func (u *User) CanViewMedia() bool {
	return u.CanViewMessages() && u.canViewMedia
}

func (u *User) CanViewCalls() bool {
	return u.canViewCalls
}

func (u *User) CanViewCallFrom() bool {
	return u.canViewCalls && u.canViewCallFrom
}

func (u *User) CanViewCallTo() bool {
	return u.canViewCalls && u.canViewCallTo
}

func (u *User) CanViewNumRecordings() bool {
	return u.canViewNumRecordings
}

func (u *User) CanPlayRecordings() bool {
	return u.canViewNumRecordings && u.canPlayRecordings
}

type Permission struct {
	maxResourceAge time.Duration
}

// ErrTooOld is returned for a resource that's more than MaxResourceAge old.
var ErrTooOld = errors.New("Cannot access this resource because its age exceeds the viewable limit")
var PermissionDenied = errors.New("You do not have permission to access that information")

func (p *Permission) MaxResourceAge() time.Duration {
	return p.maxResourceAge
}

func NewPermission(maxResourceAge time.Duration) *Permission {
	return &Permission{
		maxResourceAge: maxResourceAge,
	}
}

// TODO store in database or something
var userMap = make(map[string]*User)
var userMu sync.Mutex

// TODO fix
func AddUser(name string, u *User) {
	userMu.Lock()
	defer userMu.Unlock()
	userMap[name] = u
}

type ctxVar int

var userKey ctxVar = 0

// Auth finds the authenticating User for the request, or returns an error if
// none could be found. Auth also sets the user in the request's context.
func AuthUser(r *http.Request) (*http.Request, *User, error) {
	user, _, ok := r.BasicAuth()
	if !ok {
		return r, nil, errors.New("No user provided")
	}
	userMu.Lock()
	defer userMu.Unlock()
	if u, ok := userMap[user]; ok {
		r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		return r, u, nil
	} else {
		return r, nil, errors.New("No user named " + user)
	}
}

// GetUser returns a User stored in the request's context, if one exists.
func GetUser(r *http.Request) (*User, bool) {
	val := r.Context().Value(userKey)
	if val != nil {
		u, ok := val.(*User)
		return u, ok
	}
	return nil, false
}
