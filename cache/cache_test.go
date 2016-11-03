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
	c.AddMessagePage("npuri", mp, time.Hour)
	mp2, ok := c.GetMessagePageByURL("npuri")
	if !ok {
		t.Errorf("couldn't retrieve message page from cache")
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
	c.AddMessagePage("npuri", mp, time.Hour)
	_, ok := c.GetMessagePageByURL("npuri+badcacheget")
	if ok {
		t.Errorf("retrieved message page from cache, should have got false")
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
	c.AddMessagePage("npuri", mp, time.Nanosecond)
	_, ok := c.GetMessagePageByURL("npuri")
	if ok {
		t.Errorf("retrieved message page from cache, it should have expired")
	}
}
