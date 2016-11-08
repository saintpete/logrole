package config

import (
	"net/http"

	"golang.org/x/net/context"
)

var DefaultUser = NewUser(AllUserSettings())

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
	canViewAlerts         bool
	canViewCallbackURLs   bool
}

// UserSettings are used to define which permissions a User has. When parsing
// from YAML, any omitted fields are set to "true".
type UserSettings struct {
	// Can the user see whether a message had MMS attached?
	CanViewNumMedia bool `yaml:"can_view_num_media"`
	// Can the user view anything about a SMS/MMS message?
	CanViewMessages bool `yaml:"can_view_messages"`
	// Can the user view the message sender?
	CanViewMessageFrom bool `yaml:"can_view_message_from"`
	// Can the user view the message recipient's phone number?
	CanViewMessageTo bool `yaml:"can_view_message_to"`
	// Can the user view a SMS message body?
	CanViewMessageBody bool `yaml:"can_view_message_body"`
	// Can the user view the photos in a MMS message?
	CanViewMedia bool `yaml:"can_view_media"`

	// Can the user see how much a message cost to send?
	CanViewMessagePrice bool `yaml:"can_view_message_price"`

	// Can the user see any information about a call?
	CanViewCalls bool `yaml:"can_view_calls"`
	// Can the user view the call originator?
	CanViewCallFrom bool `yaml:"can_view_call_from"`
	// Can the user view the call recipient?
	CanViewCallTo    bool `yaml:"can_view_call_to"`
	CanViewCallPrice bool `yaml:"can_view_call_price"`
	// Can the user see whether a call has recordings attached?
	CanViewNumRecordings bool `yaml:"can_view_num_recordings"`
	// Can the user listen to recordings?
	CanPlayRecordings     bool `yaml:"can_play_recordings"`
	CanViewRecordingPrice bool `yaml:"can_view_recording_price"`
	// Can the user view metadata about a conference (sid, date created,
	// region, etc)?
	CanViewConferences bool `yaml:"can_view_conferences"`
	// Can the user view information about errors that occurred while routing
	// a call? e.g. "HTTP retrieval failure" at the callback URL.
	CanViewAlerts bool `yaml:"can_view_alerts"`
	// Can the user view a StatusCallbackURL?
	CanViewCallbackURLs bool `yaml:"can_view_callback_urls"`
}

// An alias type to avoid infinite recursion when calling UnmarshalYAML.
type yamlSettings UserSettings

// Unmarshal YAML into the UserSettings object. By default, unspecified values
// are set to true.
func (us *UserSettings) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if us == nil {
		us = new(UserSettings)
	}
	// sets everything to true
	aus := AllUserSettings()
	ys := yamlSettings(*aus)
	if err := unmarshal(&ys); err != nil {
		return err
	}
	*us = UserSettings(ys)
	return nil
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
		CanViewAlerts:         true,
		CanViewCallbackURLs:   true,
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
		canViewAlerts:         us.CanViewAlerts,
		canViewCallbackURLs:   us.CanViewCallbackURLs,
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

func (u *User) CanViewAlerts() bool {
	return u.canViewAlerts
}

func (u *User) CanViewCallbackURLs() bool {
	return u.canViewCallbackURLs
}

type ctxVar int

var userKey ctxVar = 0

// SetUser sets the User in the Request's context.
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
