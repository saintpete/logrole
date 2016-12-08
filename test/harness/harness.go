package harness

import (
	"net/http/httptest"
	"time"

	log "github.com/inconshreveable/log15"
	twilio "github.com/saintpete/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
)

var NullLogger = log.New()

func init() {
	NullLogger.SetHandler(log.DiscardHandler())
}

type ViewHarness struct {
	TestServer     *httptest.Server
	TwilioClient   *twilio.Client
	SecretKey      *[32]byte
	MaxResourceAge time.Duration
}

func ViewsClient(harness ViewHarness) views.Client {
	var c *twilio.Client
	if harness.TwilioClient == nil {
		c = twilio.NewClient("AC123", "123", nil)
	} else {
		c = harness.TwilioClient
	}
	if harness.TestServer != nil {
		c.Base = harness.TestServer.URL
	}
	if harness.SecretKey == nil {
		harness.SecretKey = services.NewRandomKey()
	}
	if harness.MaxResourceAge == 0 {
		harness.MaxResourceAge = 720 * time.Hour
	}
	return views.NewClient(NullLogger, c, harness.SecretKey, config.NewPermission(harness.MaxResourceAge))
}
