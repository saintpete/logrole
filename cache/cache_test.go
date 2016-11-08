package cache

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/kevinburke/handlers"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/test"
)

func TestEncodeDecode(t *testing.T) {
	t.Parallel()
	mp := new(twilio.MessagePage)
	if err := json.Unmarshal(test.MessageBody, mp); err != nil {
		t.Fatal(err)
	}
	c := NewCache(1, test.NullLogger)
	c.Set("npuri", mp, time.Hour)
	mp2 := new(twilio.MessagePage)
	err := c.Get("npuri", mp2)
	if err != nil {
		t.Errorf("couldn't retrieve message page from cache: %#v", err)
	}
	if !reflect.DeepEqual(mp, mp2) {
		t.Errorf("structs were not deep equal")
	}
}

func TestValueNotFound(t *testing.T) {
	t.Parallel()
	mp := new(twilio.MessagePage)
	if err := json.Unmarshal(test.MessageBody, mp); err != nil {
		t.Fatal(err)
	}
	c := NewCache(1, test.NullLogger)
	c.Set("npuri", mp, time.Hour)
	mp2 := new(twilio.MessagePage)
	err := c.Get("npuri+badcacheget", mp2)
	if err != errNotFound {
		t.Errorf("retrieved message page from cache, should have got false: %#v", err)
	}
}

func TestExpiredValueNotFound(t *testing.T) {
	t.Parallel()
	mp := new(twilio.MessagePage)
	if err := json.Unmarshal(test.MessageBody, mp); err != nil {
		t.Fatal(err)
	}
	c := NewCache(1, test.NullLogger)
	c = NewCache(1, handlers.NewLoggerLevel(log15.LvlDebug))
	c.Set("npuri", mp, time.Nanosecond)
	mp2 := new(twilio.MessagePage)
	err := c.Get("npuri", mp2)
	if err != expired {
		t.Errorf("retrieved message page from cache, it should have expired: %#v", err)
	}
}
