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
	"compress/gzip"
	"encoding/gob"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	log "github.com/inconshreveable/log15"
	twilio "github.com/kevinburke/twilio-go"
)

type Cache struct {
	log.Logger
	c  *lru.Cache
	mu sync.RWMutex
}

var errNotFound = errors.New("Key not found in cache")

func NewCache(size int, l log.Logger) *Cache {
	return &Cache{
		Logger: l,
		c:      lru.New(size),
	}
}

func (c *Cache) ConferencePagePrefix() string {
	return "conferences"
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
	return e.Page, true
}

func (c *Cache) GetConferencePageByURL(nextPage string) (*twilio.ConferencePage, bool) {
	cp := new(twilio.ConferencePage)
	if err := c.decodeValueAtKey(c.ConferencePagePrefix()+"."+nextPage, cp); err != nil {
		return nil, false
	}
	return cp, true
}

func (c *Cache) GetMessagePageByURL(nextPage string) (*twilio.MessagePage, bool) {
	mp := new(twilio.MessagePage)
	if err := c.decodeValueAtKey(c.MessagePagePrefix()+"."+nextPage, mp); err != nil {
		return nil, false
	}
	return mp, true
}

func (c *Cache) GetCallPageByURL(nextPage string) (*twilio.CallPage, bool) {
	cp := new(twilio.CallPage)
	if err := c.decodeValueAtKey(c.MessagePagePrefix()+"."+nextPage, cp); err != nil {
		return nil, false
	}
	return cp, true
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
		return errNotFound
	}
	bits, ok := val.([]byte)
	if !ok {
		c.Warn("Invalid value in cache", "val", val, "key", key)
		return errors.New("could not cast value to []byte")
	}
	c.Debug("found value in cache", "key", key, "size", len(bits))
	reader, err := gzip.NewReader(bytes.NewReader(bits))
	if err != nil {
		panic(err)
	}
	defer reader.Close()
	dec := gob.NewDecoder(reader)
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
	writer := gzip.NewWriter(&buf)
	enc := gob.NewEncoder(writer)
	if err := enc.Encode(data); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Add(key, buf.Bytes())
	c.Debug("stored data in cache", "key", key, "size", buf.Len(), "cache_size", c.c.Len())
}

func (c *Cache) AddMessagePage(npuri string, mp *twilio.MessagePage) {
	c.encAndStore(c.MessagePagePrefix()+"."+npuri, mp)
}

func (c *Cache) AddCallPage(npuri string, cp *twilio.CallPage) {
	c.encAndStore(c.CallPagePrefix()+"."+npuri, cp)
}

func (c *Cache) AddConferencePage(npuri string, cp *twilio.ConferencePage) {
	c.encAndStore(c.ConferencePagePrefix()+"."+npuri, cp)
}
