// Package views retrieves data and controls which of it is visible.
//
// This is the only package that should interact directly with Twilio - all
// other code should talk to this package to determine whether a particular
// piece of information should be visible, or not.
package views

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/golang/groupcache/singleflight"
	log "github.com/inconshreveable/log15"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/cache"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
)

// A Client retrieves resources from a backend API, and hides information that
// shouldn't be seen before returning them to the caller.
type Client interface {
	SetBasicAuth(r *http.Request)
	GetMessage(user *config.User, sid string) (*Message, error)
	GetCall(user *config.User, sid string) (*Call, error)
	GetMediaURLs(u *config.User, sid string) ([]*url.URL, error)
	GetMessagePage(user *config.User, data url.Values) (*MessagePage, error)
	GetCallPage(user *config.User, data url.Values) (*CallPage, error)
	GetNextMessagePage(user *config.User, nextPage string) (*MessagePage, error)
	GetNextCallPage(user *config.User, nextPage string) (*CallPage, error)
	GetNextRecordingPage(user *config.User, nextPage string) (*RecordingPage, error)
	GetCallRecordings(user *config.User, callSid string, data url.Values) (*RecordingPage, error)
}

type client struct {
	log.Logger
	group      singleflight.Group
	cache      *cache.Cache
	client     *twilio.Client
	secretKey  *[32]byte
	permission *config.Permission
}

// NewClient creates a new Client encapsulating the provided values.
func NewClient(l log.Logger, c *twilio.Client, secretKey *[32]byte, p *config.Permission) Client {
	return &client{
		Logger: l,
		group:  singleflight.Group{},
		// a message page is about 24k bytes compressed, 1000 entries is about
		// 25 MB. We can toggle this
		cache:      cache.NewCache(1000),
		client:     c,
		secretKey:  secretKey,
		permission: p,
	}
}

// SetBasicAuth sets the Twilio AccountSid and AuthToken on the given request.
func (vc *client) SetBasicAuth(r *http.Request) {
	r.SetBasicAuth(vc.client.AccountSid, vc.client.AuthToken)
}

// GetMessage fetches a single Message from the Twilio API, and returns any
// network or permission errors that occur.
func (vc *client) GetMessage(user *config.User, sid string) (*Message, error) {
	message, err := vc.client.Messages.Get(sid)
	if err != nil {
		return nil, err
	}
	return NewMessage(message, vc.permission, user)
}

// GetCall fetches a single Call from the Twilio API, and returns any
// network or permission errors that occur.
func (vc *client) GetCall(user *config.User, sid string) (*Call, error) {
	call, err := vc.client.Calls.Get(sid)
	if err != nil {
		return nil, err
	}
	return NewCall(call, vc.permission, user)
}

// Just make sure we get all of the media when we make a request
var mediaUrlsFilters = url.Values{
	"PageSize": []string{"100"},
}

// GetMediaURLs retrieves all media URL's for a given client, but encrypts and
// obscures them behind our image proxy first.
func (vc *client) GetMediaURLs(u *config.User, sid string) ([]*url.URL, error) {
	if u.CanViewMedia() == false {
		return nil, config.PermissionDenied
	}
	urls, err := vc.client.Messages.GetMediaURLs(sid, mediaUrlsFilters)
	if err != nil {
		return nil, err
	}
	opaqueImages := make([]*url.URL, len(urls))
	for i, u := range urls {
		enc := services.Opaque(u.String(), vc.secretKey)
		opaqueURL, err := url.Parse("/images/" + enc)
		if err != nil {
			return nil, err
		}
		opaqueImages[i] = opaqueURL
	}
	return opaqueImages, nil
}

func (vc *client) GetMessagePage(user *config.User, data url.Values) (*MessagePage, error) {
	page, err := vc.client.Messages.GetPage(data)
	if err != nil {
		return nil, err
	}
	return NewMessagePage(page, vc.permission, user)
}

func (vc *client) GetNextMessagePage(user *config.User, nextPage string) (*MessagePage, error) {
	val, err := vc.group.Do("messages."+nextPage, func() (interface{}, error) {
		if page, ok := vc.cache.GetMessagePage(nextPage); ok {
			return page, nil
		}
		page := new(twilio.MessagePage)
		if err := vc.client.GetNextPage(nextPage, page); err != nil {
			return nil, err
		}
		if page == nil {
			panic("nil page")
		}
		vc.cache.AddMessagePage(nextPage, page)
		return page, nil
	})
	if err != nil {
		return nil, err
	}
	page, ok := val.(*twilio.MessagePage)
	if !ok {
		return nil, errors.New("Could not cast fetch result to a MessagePage")
	}
	return NewMessagePage(page, vc.permission, user)
}

func (vc *client) GetCallPage(user *config.User, data url.Values) (*CallPage, error) {
	page, err := vc.client.Calls.GetPage(data)
	if err != nil {
		return nil, err
	}
	return NewCallPage(page, vc.permission, user)
}

func (vc *client) GetNextCallPage(user *config.User, nextPage string) (*CallPage, error) {
	val, err := vc.group.Do("calls."+nextPage, func() (interface{}, error) {
		if page, ok := vc.cache.GetCallPage(nextPage); ok {
			return page, nil
		}
		page := new(twilio.CallPage)
		if err := vc.client.GetNextPage(nextPage, page); err != nil {
			return nil, err
		}
		if page == nil {
			panic("nil page")
		}
		vc.cache.AddCallPage(nextPage, page)
		return page, nil
	})
	if err != nil {
		return nil, err
	}
	page, ok := val.(*twilio.CallPage)
	if !ok {
		return nil, errors.New("Could not cast fetch result to a CallPage")
	}
	return NewCallPage(page, vc.permission, user)
}

func (vc *client) GetNextRecordingPage(user *config.User, nextPage string) (*RecordingPage, error) {
	page := new(twilio.RecordingPage)
	err := vc.client.GetNextPage(nextPage, page)
	if err != nil {
		return nil, err
	}
	return NewRecordingPage(page, vc.permission, user, vc.secretKey)
}

func (vc *client) GetCallRecordings(user *config.User, callSid string, data url.Values) (*RecordingPage, error) {
	page, err := vc.client.Calls.GetRecordings(callSid, data)
	if err != nil {
		return nil, err
	}
	return NewRecordingPage(page, vc.permission, user, vc.secretKey)
}
