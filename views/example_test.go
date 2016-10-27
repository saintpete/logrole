package views_test

import (
	"fmt"
	"net/url"
	"time"

	"github.com/kevinburke/handlers"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
	"golang.org/x/net/context"
)

func Example() {
	c := twilio.NewClient("AC123", "123", nil)
	key := services.NewRandomKey()
	permission := config.NewPermission(24 * time.Hour)
	user := config.NewUser(config.AllUserSettings())

	vc := views.NewClient(handlers.Logger, c, key, permission)

	page, _ := vc.GetMessagePage(context.TODO(), user, url.Values{})
	for _, msg := range page.Messages() {
		sid, _ := msg.Sid()
		fmt.Println(sid)
	}
}
