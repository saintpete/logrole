package config

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

type User struct {
	canViewNumMedia       bool
	canViewMessages       bool
	canViewMessageFrom    bool
	canViewMessageTo      bool
	canViewMessageBody    bool
	canViewMessagePrice   bool
	canViewMedia          bool
	canViewCalls          bool
	canViewCallFrom       bool
	canViewCallTo         bool
	canViewCallPrice      bool
	canViewNumRecordings  bool
	canPlayRecordings     bool
	canViewRecordingPrice bool
	canViewConferences    bool
}

// UserSettings are used to define which permissions a User has.
type UserSettings struct {
	CanViewNumMedia     bool
	CanViewMessages     bool
	CanViewMessageFrom  bool
	CanViewMessageTo    bool
	CanViewMessageBody  bool
	CanViewMessagePrice bool
	CanViewMedia        bool
	CanViewCalls        bool
	CanViewCallFrom     bool
	CanViewCallTo       bool
	CanViewCallPrice    bool
	// Can the user see whether a call has recordings attached?
	CanViewNumRecordings bool
	// Can the user listen to recordings?
	CanPlayRecordings     bool
	CanViewRecordingPrice bool
	// Can the user view metadata about a conference (sid, date created,
	// region, etc)?
	CanViewConferences bool
}

// AllUserSettings returns a UserSettings value with the widest possible set of
// permissions.
func AllUserSettings() *UserSettings {
	return &UserSettings{
		CanViewNumMedia:       true,
		CanViewMessages:       true,
		CanViewMessageFrom:    true,
		CanViewMessageTo:      true,
		CanViewMessageBody:    true,
		CanViewMessagePrice:   true,
		CanViewMedia:          true,
		CanViewCalls:          true,
		CanViewCallFrom:       true,
		CanViewCallTo:         true,
		CanViewCallPrice:      true,
		CanViewNumRecordings:  true,
		CanPlayRecordings:     true,
		CanViewRecordingPrice: true,
		CanViewConferences:    true,
	}
}

// NewUser creates a new User with the given settings.
func NewUser(us *UserSettings) *User {
	if us == nil {
		us = &UserSettings{}
	}
	return &User{
		canViewNumMedia:       us.CanViewNumMedia,
		canViewMessages:       us.CanViewMessages,
		canViewMessageFrom:    us.CanViewMessageFrom,
		canViewMessageTo:      us.CanViewMessageTo,
		canViewMessageBody:    us.CanViewMessageBody,
		canViewMessagePrice:   us.CanViewMessagePrice,
		canViewMedia:          us.CanViewMedia,
		canViewCalls:          us.CanViewCalls,
		canViewCallFrom:       us.CanViewCallFrom,
		canViewCallTo:         us.CanViewCallTo,
		canViewCallPrice:      us.CanViewCallPrice,
		canViewNumRecordings:  us.CanViewNumRecordings,
		canPlayRecordings:     us.CanPlayRecordings,
		canViewRecordingPrice: us.CanViewRecordingPrice,
		canViewConferences:    us.CanViewConferences,
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

func (u *User) CanViewMessagePrice() bool {
	return u.CanViewMessages() && u.canViewMessagePrice
}

func (u *User) CanViewMedia() bool {
	return u.CanViewMessages() && u.canViewMedia
}

func (u *User) CanViewCalls() bool {
	return u.canViewCalls
}

func (u *User) CanViewCallFrom() bool {
	return u.CanViewCalls() && u.canViewCallFrom
}

func (u *User) CanViewCallTo() bool {
	return u.CanViewCalls() && u.canViewCallTo
}

func (u *User) CanViewCallPrice() bool {
	return u.CanViewCalls() && u.canViewCallPrice
}

func (u *User) CanViewNumRecordings() bool {
	return u.canViewNumRecordings
}

func (u *User) CanPlayRecordings() bool {
	return u.canPlayRecordings
}

func (u *User) CanViewRecordingPrice() bool {
	return u.canViewRecordingPrice
}

func (u *User) CanViewConferences() bool {
	return u.canViewConferences
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
// none could be found. Auth also sets the user in the request's context and
// returns it.
func AuthUser(r *http.Request) (*http.Request, *User, error) {
	user, _, ok := r.BasicAuth()
	if !ok {
		return r, nil, errors.New("No user provided")
	}
	userMu.Lock()
	defer userMu.Unlock()
	if u, ok := userMap[user]; ok {
		r = SetUser(r, u)
		return r, u, nil
	} else {
		return r, nil, errors.New("No user named " + user)
	}
}

func SetUser(r *http.Request, u *User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userKey, u))
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
