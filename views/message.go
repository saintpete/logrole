package views

import (
	"errors"
	"net/url"
	"time"

	types "github.com/kevinburke/go-types"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
)

type Message struct {
	user    *config.User
	message *twilio.Message
}

// CanViewProperty returns true if the caller can access the given property.
// CanViewProperty panics if the property does not exist. The input is
// case-sensitive; "MessagingServiceSid" is the correct casing.
func (m *Message) CanViewProperty(property string) bool {
	if m.user == nil {
		return false
	}
	switch property {
	case "Sid", "DateCreated", "DateUpdated", "MessagingServiceSid",
		"Status", "Direction", "Price", "PriceUnit":
		return m.user.CanViewMessages()
	case "NumMedia":
		return m.user.CanViewNumMedia()
	case "From":
		return m.user.CanViewMessageFrom()
	case "To":
		return m.user.CanViewMessageTo()
	case "Body", "NumSegments":
		return m.user.CanViewMessageBody()
	default:
		panic("unknown property " + property)
	}
}

func (m *Message) NumMedia() (twilio.NumMedia, error) {
	if m.user.CanViewNumMedia() {
		return m.message.NumMedia, nil
	} else {
		return 0, config.PermissionDenied
	}
}

func (m *Message) Sid() (string, error) {
	if m.user.CanViewMessages() {
		return m.message.Sid, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (m *Message) DateCreated() (twilio.TwilioTime, error) {
	if m.user.CanViewMessages() {
		return m.message.DateCreated, nil
	} else {
		return twilio.TwilioTime{}, config.PermissionDenied
	}
}

func (m *Message) From() (twilio.PhoneNumber, error) {
	if m.user.CanViewMessageFrom() {
		return m.message.From, nil
	} else {
		return twilio.PhoneNumber(""), config.PermissionDenied
	}
}

func (m *Message) To() (twilio.PhoneNumber, error) {
	if m.user.CanViewMessageTo() {
		return m.message.To, nil
	} else {
		return twilio.PhoneNumber(""), config.PermissionDenied
	}
}

func (m *Message) MessagingServiceSid() (types.NullString, error) {
	if m.CanViewProperty("MessagingServiceSid") {
		return m.message.MessagingServiceSid, nil
	} else {
		return types.NullString{}, config.PermissionDenied
	}
}

func (m *Message) Status() (twilio.Status, error) {
	if m.CanViewProperty("Status") {
		return m.message.Status, nil
	} else {
		return twilio.Status(""), config.PermissionDenied
	}
}

func (m *Message) Direction() (twilio.Direction, error) {
	if m.CanViewProperty("Direction") {
		return m.message.Direction, nil
	} else {
		return twilio.Direction(""), config.PermissionDenied
	}
}

func (m *Message) Price() (string, error) {
	if m.CanViewProperty("Price") {
		return m.message.Price, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (m *Message) PriceUnit() (string, error) {
	if m.CanViewProperty("PriceUnit") {
		return m.message.PriceUnit, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (m *Message) FriendlyPrice() (string, error) {
	if m.CanViewProperty("Price") && m.CanViewProperty("PriceUnit") {
		return m.message.FriendlyPrice(), nil
	} else {
		return "", config.PermissionDenied
	}
}

func (m *Message) Body() (string, error) {
	if m.CanViewProperty("Body") {
		return m.message.Body, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (m *Message) NumSegments() (twilio.Segments, error) {
	if m.CanViewProperty("NumSegments") {
		return m.message.NumSegments, nil
	} else {
		return 0, config.PermissionDenied
	}
}

func (m *Message) CanViewMedia() bool {
	// Hack - a separate function since this is not a property on the object.
	return m.user != nil && m.user.CanViewMedia()
}

func NewMessage(msg *twilio.Message, p *config.Permission, u *config.User) (*Message, error) {
	if msg.DateCreated.Valid == false {
		return nil, errors.New("Invalid CreatedAt date for message")
	}
	oldest := time.Now().UTC().Add(-1 * p.MaxResourceAge())
	if msg.DateCreated.Time.Before(oldest) {
		return nil, config.ErrTooOld
	}
	return &Message{user: u, message: msg}, nil
}

// Just make sure we get all of the media when we make a request
var mediaUrlsFilters = url.Values{
	"PageSize": []string{"100"},
}

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
