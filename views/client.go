package views

import (
	log "github.com/inconshreveable/log15"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
)

// A Client retrieves resources from the Twilio API, and hides information that
// shouldn't be seen before returning them to the caller.
type Client struct {
	log.Logger
	client     *twilio.Client
	secretKey  *[32]byte
	permission *config.Permission
}

func NewClient(l log.Logger, client *twilio.Client, secretKey *[32]byte, p *config.Permission) *Client {
	return &Client{
		Logger:     l,
		client:     client,
		secretKey:  secretKey,
		permission: p,
	}
}
