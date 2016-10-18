package views

import (
	"errors"
	"net/url"
	"time"

	types "github.com/kevinburke/go-types"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
)

type Message struct {
	user    *config.User
	message *twilio.Message
}

type MessagePage struct {
	messages    []*Message
	nextPageURI types.NullString
}

func (mp *MessagePage) Messages() []*Message {
	return mp.messages
}

func (mp *MessagePage) NextPageURI() types.NullString {
	return mp.nextPageURI
}

const showAllColumnsOnEmptyPage = true

// ShowHeader returns true if we should show the table header in the message
// list view. This is true if the user is allowed to view the fieldName on any
// message in the list, and true if there are no messages.
func (mp *MessagePage) ShowHeader(fieldName string) bool {
	if mp == nil {
		return true
	}
	msgs := mp.Messages()
	if len(msgs) == 0 {
		return showAllColumnsOnEmptyPage
	}
	for _, message := range msgs {
		if message.CanViewProperty(fieldName) {
			return true
		}
	}
	return false
}

func NewMessagePage(mp *twilio.MessagePage, p *config.Permission, u *config.User) (*MessagePage, error) {
	messages := make([]*Message, 0)
	for _, message := range mp.Messages {
		msg, err := NewMessage(message, p, u)
		if err == config.ErrTooOld || err == config.PermissionDenied {
			continue
		}
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return &MessagePage{messages: messages, nextPageURI: mp.NextPageURI}, nil
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
