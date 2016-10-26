// Package cache caches Twilio API requests for fast loading.
//
// Fetching a second page of resources from Twilio can be extremely slow - one
// second or more. Often we know the URL we want to fetch in advance - the
// first page of Messages or Calls, and any next_page_uri as soon as a user
// retrieves any individual page. Fetching the page and caching it can greatly
// improve latency.
package cache

// TODO work on the API's for storing/retrieving messages.

import (
	"bytes"
	"encoding/gob"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	log "github.com/inconshreveable/log15"
	"github.com/kevinburke/handlers"
	twilio "github.com/kevinburke/twilio-go"
)

const debug = false

type Cache struct {
	log.Logger
	c  *lru.Cache
	mu sync.RWMutex
}

func NewCache(size int) *Cache {
	l := handlers.NewLogger()
	if debug {
		l.SetHandler(log.LvlFilterHandler(log.LvlDebug, l.GetHandler()))
	}
	return &Cache{
		Logger: l,
		c:      lru.New(size),
	}
}

func (c *Cache) MessagePagePrefix() string {
	return "messages"
}

func (c *Cache) CallPagePrefix() string {
	return "calls"
}

type ExpiringMessagePage struct {
	Expiry time.Time
	Page   *twilio.MessagePage
}

type ExpiringCallPage struct {
	Expiry time.Time
	Page   *twilio.CallPage
}

type ExpiringConferencePage struct {
	Expiry time.Time
	Page   *twilio.ConferencePage
}

// GetMessagePageByValues retrieves messages for given set of query values.
func (c *Cache) GetMessagePageByValues(data url.Values) (*twilio.MessagePage, bool) {
	key := "expiring_messages." + data.Encode()
	e := new(ExpiringMessagePage)
	if err := c.decodeValueAtKey(key, e); err != nil {
		return nil, false
	}
	if time.Since(e.Expiry) > 0 {
		c.remove(key)
		return nil, false
	}
	c.Debug("found expiring message page in cache", "key", key, "size", c.c.Len())
	return e.Page, true
}

func (c *Cache) GetMessagePageByURL(nextPage string) (*twilio.MessagePage, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.c.Get(c.MessagePagePrefix() + "." + nextPage)
	if !ok {
		return nil, false
	}
	bits, ok := val.([]byte)
	if !ok {
		return nil, false
	}
	dec := gob.NewDecoder(bytes.NewReader(bits))
	mp := new(twilio.MessagePage)
	if err := dec.Decode(mp); err != nil {
		return nil, false
	}
	c.Debug("found message page in cache", "key", nextPage, "size", c.c.Len())
	return mp, true
}

func (c *Cache) GetCallPageByURL(nextPage string) (*twilio.CallPage, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.c.Get(c.CallPagePrefix() + "." + nextPage)
	if !ok {
		return nil, false
	}
	bits, ok := val.([]byte)
	if !ok {
		return nil, false
	}
	dec := gob.NewDecoder(bytes.NewReader(bits))
	mp := new(twilio.CallPage)
	if err := dec.Decode(mp); err != nil {
		return nil, false
	}
	c.Debug("found call page in cache", "key", nextPage, "size", c.c.Len())
	return mp, true
}

func (c *Cache) GetCallPageByValues(data url.Values) (*twilio.CallPage, bool) {
	key := "expiring_calls." + data.Encode()
	e := new(ExpiringCallPage)
	if err := c.decodeValueAtKey(key, e); err != nil {
		return nil, false
	}
	if time.Since(e.Expiry) > 0 {
		c.remove(key)
		return nil, false
	}
	c.Debug("found expiring call page in cache", "key", key, "size", c.c.Len())
	return e.Page, true
}

func (c *Cache) remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Remove(key)
}

func (c *Cache) decodeValueAtKey(key string, e interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.c.Get(key)
	if !ok {
		return errors.New("key not found")
	}
	bits, ok := val.([]byte)
	if !ok {
		c.Warn("Invalid value in cache", "val", val, "key", key)
		return errors.New("could not cast value to []byte")
	}
	dec := gob.NewDecoder(bytes.NewReader(bits))
	return dec.Decode(e)
}

// GetConferencePageByValues retrieves messages for given set of query values.
func (c *Cache) GetConferencePageByValues(data url.Values) (*twilio.ConferencePage, bool) {
	key := "expiring_conferences." + data.Encode()
	e := new(ExpiringConferencePage)
	if err := c.decodeValueAtKey(key, e); err != nil {
		return nil, false
	}
	if time.Since(e.Expiry) > 0 {
		c.remove(key)
		return nil, false
	}
	c.Debug("found expiring conference page in cache", "key", key, "size", c.c.Len())
	return e.Page, true
}

// AddExpiringMessagePage caches mp at the given key for the provided duration.
// Use GetMessagePageByValues to retrieve it.
func (c *Cache) AddExpiringMessagePage(key string, valid time.Duration, mp *twilio.MessagePage) {
	e := &ExpiringMessagePage{
		Expiry: time.Now().UTC().Add(valid),
		Page:   mp,
	}
	c.encAndStore("expiring_messages."+key, e)
}

func (c *Cache) AddExpiringConferencePage(key string, valid time.Duration, mp *twilio.ConferencePage) {
	e := &ExpiringConferencePage{
		Expiry: time.Now().UTC().Add(valid),
		Page:   mp,
	}
	c.encAndStore("expiring_conferences."+key, e)
}

// AddExpiringCallPage caches mp at the given key for the provided duration.
// Use GetCallPageByValues to retrieve it.
func (c *Cache) AddExpiringCallPage(key string, valid time.Duration, cp *twilio.CallPage) {
	e := &ExpiringCallPage{
		Expiry: time.Now().UTC().Add(valid),
		Page:   cp,
	}
	c.encAndStore("expiring_calls."+key, e)
}

func (c *Cache) encAndStore(key string, data interface{}) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		panic(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Add(key, buf.Bytes())
	c.Debug("stored data in cache", "key", key, "size", c.c.Len())
}

func (c *Cache) AddMessagePage(npuri string, mp *twilio.MessagePage) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(mp)
	if err != nil {
		panic(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Add(c.MessagePagePrefix()+"."+npuri, buf.Bytes())
	c.Debug("stored message page in cache", "key", npuri, "size", c.c.Len())
}

func (c *Cache) AddCallPage(npuri string, mp *twilio.CallPage) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(mp)
	if err != nil {
		panic(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Add(c.CallPagePrefix()+"."+npuri, buf.Bytes())
	c.Debug("stored call page in cache", "key", npuri, "size", c.c.Len())
}
