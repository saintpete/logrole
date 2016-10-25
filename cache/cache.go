package cache

import (
	"bytes"
	"encoding/gob"
	"net/url"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	log "github.com/inconshreveable/log15"
	twilio "github.com/kevinburke/twilio-go"
)

const debug = false

type Cache struct {
	log.Logger
	c  *lru.Cache
	mu sync.RWMutex
}

func NewCache(size int) *Cache {
	l := log.New()
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

// GetMessagePageByValues retrieves
func (c *Cache) GetMessagePageByValues(data url.Values) (*twilio.MessagePage, bool) {
	key := "expiring_messages." + data.Encode()
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.c.Get(key)
	if !ok {
		return nil, false
	}
	bits, ok := val.([]byte)
	if !ok {
		c.Warn("Invalid value in cache", "val", val, "key", key)
		return nil, false
	}
	dec := gob.NewDecoder(bytes.NewReader(bits))
	e := new(ExpiringMessagePage)
	if err := dec.Decode(e); err != nil {
		return nil, false
	}
	if time.Since(e.Expiry) > 0 {
		c.c.Remove(key)
		return nil, false
	}
	c.Debug("found expiring message page in cache", "key", key)
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
	c.Debug("found message page in cache", "key", nextPage)
	return mp, true
}

func (c *Cache) GetCallPage(key string) (*twilio.CallPage, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.c.Get(c.CallPagePrefix() + "." + key)
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
	c.Debug("found call page in cache", "key", key)
	return mp, true
}

func (c *Cache) AddExpiringMessagePage(key string, valid time.Duration, mp *twilio.MessagePage) {
	e := &ExpiringMessagePage{
		Expiry: time.Now().UTC().Add(valid),
		Page:   mp,
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(e)
	if err != nil {
		panic(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Add("expiring_messages."+key, buf.Bytes())
	c.Debug("stored message page in cache", "key", key)
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
	c.Debug("stored message page in cache", "key", npuri)
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
	c.Debug("stored call page in cache", "key", npuri)
}
