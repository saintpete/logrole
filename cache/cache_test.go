package cache

import (
	"encoding/json"
	"reflect"
	"testing"

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
	c.AddMessagePage("npuri", mp)
	mp2, ok := c.GetMessagePageByURL("npuri")
	if !ok {
		t.Errorf("couldn't retrieve message page from cache")
	}
	if !reflect.DeepEqual(mp, mp2) {
		t.Errorf("structs were not deep equal")
	}
}
