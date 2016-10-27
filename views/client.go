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
	"strconv"
	"sync"
	"time"

	"github.com/golang/groupcache/singleflight"
	log "github.com/inconshreveable/log15"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/cache"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"golang.org/x/net/context"
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
	GetNextConferencePage(user *config.User, nextPage string) (*ConferencePage, error)
	GetNextRecordingPage(user *config.User, nextPage string) (*RecordingPage, error)
	GetCallRecordings(user *config.User, callSid string, data url.Values) (*RecordingPage, error)
	GetConferencePage(user *config.User, data url.Values) (*ConferencePage, error)
	CacheCommonQueries(uint, <-chan bool)
	IsTwilioNumber(num twilio.PhoneNumber) bool
}

type client struct {
	log.Logger
	group      singleflight.Group
	cache      *cache.Cache
	client     *twilio.Client
	secretKey  *[32]byte
	permission *config.Permission
	numbers    map[twilio.PhoneNumber]bool
	numbersMu  sync.RWMutex
}

// this allows about 8k entries in the cache
const cacheSizeMB = 25
const averageCacheEntryBytes = 3000

// NewClient creates a new Client encapsulating the provided values.
func NewClient(l log.Logger, c *twilio.Client, secretKey *[32]byte, p *config.Permission) Client {
	vc := &client{
		Logger:     l,
		group:      singleflight.Group{},
		cache:      cache.NewCache(cacheSizeMB * 1024 * 1024 / averageCacheEntryBytes),
		client:     c,
		secretKey:  secretKey,
		permission: p,
	}
	// TODO - would prefer to have another way to start this
	if vc.client != nil {
		go vc.getNumbers()
		go vc.refreshNumberMap(30 * time.Second)
	}
	return vc
}

func (vc *client) getNumbers() {
	iter := vc.client.IncomingNumbers.GetPageIterator(nil)
	size, count := 0, 0
	mp := make(map[twilio.PhoneNumber]bool)
	for count < 200 {
		page, err := iter.Next(context.Background())
		if err == twilio.NoMoreResults {
			break
		}
		if err != nil {
			return
		}
		for _, pn := range page.IncomingPhoneNumbers {
			mp[pn.PhoneNumber] = true
			size++
		}
		count++
	}
	vc.numbersMu.Lock()
	vc.numbers = mp
	vc.numbersMu.Unlock()
	vc.Debug("Updated phone number map", "size", size)
}

func (vc *client) refreshNumberMap(interval time.Duration) {
	tick := time.NewTicker(interval)
	for range tick.C {
		go vc.getNumbers()
	}
}

// SetBasicAuth sets the Twilio AccountSid and AuthToken on the given request.
func (vc *client) SetBasicAuth(r *http.Request) {
	r.SetBasicAuth(vc.client.AccountSid, vc.client.AuthToken)
}

// GetMessage fetches a single Message from the Twilio API, and returns any
// network or permission errors that occur.
func (vc *client) GetMessage(user *config.User, sid string) (*Message, error) {
	message, err := vc.client.Messages.Get(context.TODO(), sid)
	if err != nil {
		return nil, err
	}
	return NewMessage(message, vc.permission, user)
}

// GetCall fetches a single Call from the Twilio API, and returns any
// network or permission errors that occur.
func (vc *client) GetCall(user *config.User, sid string) (*Call, error) {
	call, err := vc.client.Calls.Get(context.TODO(), sid)
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
	urls, err := vc.client.Messages.GetMediaURLs(context.TODO(), sid, mediaUrlsFilters)
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

func (vc *client) getAndCacheMessage(data url.Values) (*twilio.MessagePage, error) {
	page, err := vc.client.Messages.GetPage(context.TODO(), data)
	if err != nil {
		return nil, err
	}
	vc.cache.AddExpiringMessagePage(data.Encode(), 30*time.Second, page)
	return page, nil
}

func (vc *client) getAndCacheConference(data url.Values) (*twilio.ConferencePage, error) {
	page, err := vc.client.Conferences.GetPage(context.TODO(), data)
	if err != nil {
		return nil, err
	}
	vc.cache.AddExpiringConferencePage(data.Encode(), 30*time.Second, page)
	return page, nil
}

func (vc *client) getAndCacheCall(data url.Values) (*twilio.CallPage, error) {
	page, err := vc.client.Calls.GetPage(context.TODO(), data)
	if err != nil {
		return nil, err
	}
	vc.cache.AddExpiringCallPage(data.Encode(), 30*time.Second, page)
	return page, nil
}

func (vc *client) GetMessagePage(user *config.User, data url.Values) (*MessagePage, error) {
	val, err := vc.group.Do("messages."+data.Encode(), func() (interface{}, error) {
		if page, ok := vc.cache.GetMessagePageByValues(data); ok {
			return page, nil
		}
		page, err := vc.getAndCacheMessage(data)
		return page, err
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

func (vc *client) GetConferencePage(user *config.User, data url.Values) (*ConferencePage, error) {
	val, err := vc.group.Do("conferences."+data.Encode(), func() (interface{}, error) {
		if page, ok := vc.cache.GetConferencePageByValues(data); ok {
			return page, nil
		}
		page, err := vc.getAndCacheConference(data)
		return page, err
	})
	if err != nil {
		return nil, err
	}
	page, ok := val.(*twilio.ConferencePage)
	if !ok {
		return nil, errors.New("Could not cast fetch result to a ConferencePage")
	}
	return NewConferencePage(page, vc.permission, user)
}

func (vc *client) GetNextMessagePage(user *config.User, nextPage string) (*MessagePage, error) {
	val, err := vc.group.Do("messages."+nextPage, func() (interface{}, error) {
		if page, ok := vc.cache.GetMessagePageByURL(nextPage); ok {
			return page, nil
		}
		page := new(twilio.MessagePage)
		if err := vc.client.GetNextPage(context.TODO(), nextPage, page); err != nil {
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
	val, err := vc.group.Do("calls."+data.Encode(), func() (interface{}, error) {
		if page, ok := vc.cache.GetCallPageByValues(data); ok {
			return page, nil
		}
		page, err := vc.getAndCacheCall(data)
		return page, err
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

func (vc *client) GetNextConferencePage(user *config.User, nextPage string) (*ConferencePage, error) {
	val, err := vc.group.Do("conferences."+nextPage, func() (interface{}, error) {
		if page, ok := vc.cache.GetConferencePageByURL(nextPage); ok {
			return page, nil
		}
		page := new(twilio.ConferencePage)
		if err := vc.client.GetNextPage(context.TODO(), nextPage, page); err != nil {
			return nil, err
		}
		if page == nil {
			panic("nil page")
		}
		vc.cache.AddConferencePage(nextPage, page)
		return page, nil
	})
	if err != nil {
		return nil, err
	}
	page, ok := val.(*twilio.ConferencePage)
	if !ok {
		return nil, errors.New("Could not cast fetch result to a ConferencePage")
	}
	return NewConferencePage(page, vc.permission, user)
}

func (vc *client) GetNextCallPage(user *config.User, nextPage string) (*CallPage, error) {
	val, err := vc.group.Do("calls."+nextPage, func() (interface{}, error) {
		if page, ok := vc.cache.GetCallPageByURL(nextPage); ok {
			return page, nil
		}
		page := new(twilio.CallPage)
		if err := vc.client.GetNextPage(context.TODO(), nextPage, page); err != nil {
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
	err := vc.client.GetNextPage(context.TODO(), nextPage, page)
	if err != nil {
		return nil, err
	}
	return NewRecordingPage(page, vc.permission, user, vc.secretKey)
}

func (vc *client) GetCallRecordings(user *config.User, callSid string, data url.Values) (*RecordingPage, error) {
	page, err := vc.client.Calls.GetRecordings(context.TODO(), callSid, data)
	if err != nil {
		return nil, err
	}
	return NewRecordingPage(page, vc.permission, user, vc.secretKey)
}

func (vc *client) CacheCommonQueries(pageSize uint, doneCh <-chan bool) {
	timeout := time.After(1 * time.Millisecond)
	ps := strconv.FormatUint(uint64(pageSize), 10)
	data := url.Values{"PageSize": []string{ps}}
	for {
		select {
		case <-timeout:
			go vc.getAndCacheMessage(data)
			go vc.getAndCacheCall(data)
			go vc.getAndCacheConference(data)
		case <-doneCh:
			return
		}
		timeout = time.After(30 * time.Second)
	}
}

func (vc *client) IsTwilioNumber(num twilio.PhoneNumber) bool {
	vc.numbersMu.RLock()
	_, ok := vc.numbers[num]
	vc.numbersMu.RUnlock()
	return ok
}
