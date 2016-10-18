// Package views retrieves data and controls which of it is visible.
//
// This is the only package that should interact directly with Twilio - all
// other code should talk to this package to determine whether a particular
// piece of information should be visible, or not.
package views

import (
	"errors"
	"net/url"

	log "github.com/inconshreveable/log15"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
)

// A Client retrieves resources from the Twilio API, and hides information that
// shouldn't be seen before returning them to the caller.
type Client struct {
	log.Logger
	client     *twilio.Client
	secretKey  *[32]byte
	permission *config.Permission
}

// NewClient creates a new Client encapsulating the provided values.
func NewClient(l log.Logger, client *twilio.Client, secretKey *[32]byte, p *config.Permission) *Client {
	return &Client{
		Logger:     l,
		client:     client,
		secretKey:  secretKey,
		permission: p,
	}
}

// GetMessage fetches a single Message from the Twilio API, and returns any
// network or permission errors that occur.
func (vc *Client) GetMessage(user *config.User, sid string) (*Message, error) {
	message, err := vc.client.Messages.Get(sid)
	if err != nil {
		return nil, err
	}
	return NewMessage(message, vc.permission, user)
}

// GetMediaURLs retrieves all media URL's for a given client, but encrypts and
// obscures them behind our image proxy first.
func (vc *Client) GetMediaURLs(sid string) ([]*url.URL, error) {
	urls, err := vc.client.Messages.GetMediaURLs(sid, mediaUrlsFilters)
	if err != nil {
		return nil, err
	}
	opaqueImages := make([]*url.URL, len(urls))
	for i, u := range urls {
		enc, err := services.Opaque(u.String(), vc.secretKey)
		if err != nil {
			vc.Warn("Could not encrypt media URL", "raw", u.String(), "err", err)
			return nil, errors.New("Could not encode URL as a string")
		}
		opaqueURL, err := url.Parse("/images/" + enc)
		if err != nil {
			return nil, err
		}
		opaqueImages[i] = opaqueURL
	}
	return opaqueImages, nil
}

func (vc *Client) GetMessagePage(user *config.User, data url.Values) (*MessagePage, error) {
	page, err := vc.client.Messages.GetPage(data)
	if err != nil {
		return nil, err
	}
	return NewMessagePage(page, vc.permission, user)
}

func (vc *Client) GetNextMessagePage(user *config.User, nextPage string) (*MessagePage, error) {
	page := new(twilio.MessagePage)
	err := vc.client.GetNextPage(nextPage, page)
	if err != nil {
		return nil, err
	}
	return NewMessagePage(page, vc.permission, user)
}
