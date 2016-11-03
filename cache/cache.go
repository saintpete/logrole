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

var expired = errors.New("expired")
var errNotFound = errors.New("Key not found in cache")

func NewCache(size int, l log.Logger) *Cache {
	return &Cache{
		Logger: l,
		c:      lru.New(size),
	}
}

// enc gob.Encodes + gzips data. do not try to gob.Encode an interface
func enc(data interface{}) []byte {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	ec := gob.NewEncoder(writer)
	if err := ec.Encode(data); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (c *Cache) store(key string, e *ExpiringData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Add(key, e)
	c.Debug("stored data in cache", "key", key, "size", len(e.Data), "cache_size", c.c.Len())
}

func (c *Cache) decodeValueAtKey(key string, i interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.c.Get(key)
	if !ok {
		return errNotFound
	}
	e, ok := val.(*ExpiringData)
	if !ok {
		c.Warn("Invalid value in cache", "val", val, "key", key)
		return errors.New("could not cast value to ExpiringData")
	}
	if since := time.Since(e.Expires); since > 0 {
		c.Debug("found expired value in cache", "key", key, "expired_ago", since)
		c.c.Remove(key)
		return expired
	}
	c.Debug("found value in cache", "key", key, "size", len(e.Data))
	reader, err := gzip.NewReader(bytes.NewReader(e.Data))
	if err != nil {
		panic(err)
	}
	defer reader.Close()
	dec := gob.NewDecoder(reader)
	return dec.Decode(i)
}

var alertPagePrefix = "alerts"
var callPagePrefix = "calls"
var messagePagePrefix = "messages"
var conferencePagePrefix = "conferences"

type ExpiringData struct {
	Expires time.Time
	Data    []byte // call enc() to get an encoded value
}

// GetMessagePageByValues retrieves messages for given set of query values.
func (c *Cache) GetMessagePageByValues(data url.Values) (*twilio.MessagePage, bool) {
	key := "expiring_messages." + data.Encode()
	e := new(twilio.MessagePage)
	if err := c.decodeValueAtKey(key, e); err != nil {
		return nil, false
	}
	return e, true
}

func (c *Cache) GetConferencePageByURL(nextPage string) (*twilio.ConferencePage, bool) {
	cp := new(twilio.ConferencePage)
	if err := c.decodeValueAtKey(conferencePagePrefix+"."+nextPage, cp); err != nil {
		return nil, false
	}
	return cp, true
}

func (c *Cache) GetMessagePageByURL(nextPage string) (*twilio.MessagePage, bool) {
	mp := new(twilio.MessagePage)
	if err := c.decodeValueAtKey(messagePagePrefix+"."+nextPage, mp); err != nil {
		return nil, false
	}
	return mp, true
}

func (c *Cache) GetCallPageByURL(nextPage string) (*twilio.CallPage, bool) {
	cp := new(twilio.CallPage)
	if err := c.decodeValueAtKey(callPagePrefix+"."+nextPage, cp); err != nil {
		return nil, false
	}
	return cp, true
}

func (c *Cache) GetAlertPageByURL(nextPage string) (*twilio.AlertPage, bool) {
	ap := new(twilio.AlertPage)
	if err := c.decodeValueAtKey(alertPagePrefix+"."+nextPage, ap); err != nil {
		return nil, false
	}
	return ap, true
}

func (c *Cache) GetCallPageByValues(data url.Values) (*twilio.CallPage, bool) {
	key := "expiring_calls." + data.Encode()
	cp := new(twilio.CallPage)
	if err := c.decodeValueAtKey(key, cp); err != nil {
		return nil, false
	}
	return cp, true
}

func (c *Cache) GetAlertPageByValues(data url.Values) (*twilio.AlertPage, bool) {
	key := "expiring_alerts." + data.Encode()
	ap := new(twilio.AlertPage)
	if err := c.decodeValueAtKey(key, ap); err != nil {
		return nil, false
	}
	return ap, true
}

// GetConferencePageByValues retrieves messages for given set of query values.
func (c *Cache) GetConferencePageByValues(data url.Values) (*twilio.ConferencePage, bool) {
	key := "expiring_conferences." + data.Encode()
	cp := new(twilio.ConferencePage)
	if err := c.decodeValueAtKey(key, cp); err != nil {
		return nil, false
	}
	return cp, true
}

// AddExpiringMessagePage caches mp at the given key for the provided duration.
// Use GetMessagePageByValues to retrieve it.
func (c *Cache) AddExpiringMessagePage(key string, mp *twilio.MessagePage, valid time.Duration) {
	bits := enc(mp)
	e := &ExpiringData{
		Expires: time.Now().UTC().Add(valid),
		Data:    bits,
	}
	c.store("expiring_messages."+key, e)
}

func (c *Cache) AddExpiringAlertPage(key string, mp *twilio.AlertPage, valid time.Duration) {
	bits := enc(mp)
	e := &ExpiringData{
		Expires: time.Now().UTC().Add(valid),
		Data:    bits,
	}
	c.store("expiring_alerts."+key, e)
}

func (c *Cache) AddExpiringConferencePage(key string, cp *twilio.ConferencePage, valid time.Duration) {
	bits := enc(cp)
	e := &ExpiringData{
		Expires: time.Now().UTC().Add(valid),
		Data:    bits,
	}
	c.store("expiring_conferences."+key, e)
}

// AddExpiringCallPage caches mp at the given key for the provided duration.
// Use GetCallPageByValues to retrieve it.
func (c *Cache) AddExpiringCallPage(key string, cp *twilio.CallPage, valid time.Duration) {
	bits := enc(cp)
	e := &ExpiringData{
		Expires: time.Now().UTC().Add(valid),
		Data:    bits,
	}
	c.store("expiring_calls."+key, e)
}

func (c *Cache) AddAlertPage(npuri string, mp *twilio.AlertPage, valid time.Duration) {
	bits := enc(mp)
	e := &ExpiringData{
		Expires: time.Now().UTC().Add(valid),
		Data:    bits,
	}
	c.store(alertPagePrefix+"."+npuri, e)
}

func (c *Cache) AddMessagePage(npuri string, mp *twilio.MessagePage, valid time.Duration) {
	bits := enc(mp)
	e := &ExpiringData{
		Expires: time.Now().UTC().Add(valid),
		Data:    bits,
	}
	c.store(messagePagePrefix+"."+npuri, e)
}

func (c *Cache) AddCallPage(npuri string, cp *twilio.CallPage, valid time.Duration) {
	bits := enc(cp)
	e := &ExpiringData{
		Expires: time.Now().UTC().Add(valid),
		Data:    bits,
	}
	c.store(callPagePrefix+"."+npuri, e)
}

func (c *Cache) AddConferencePage(npuri string, cp *twilio.ConferencePage, valid time.Duration) {
	bits := enc(cp)
	e := &ExpiringData{
		Expires: time.Now().UTC().Add(valid),
		Data:    bits,
	}
	c.store(conferencePagePrefix+"."+npuri, e)
}
