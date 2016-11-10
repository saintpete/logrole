package views

import (
	"errors"

	types "github.com/kevinburke/go-types"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
)

type IncomingNumberPage struct {
	numbers         []*IncomingNumber
	nextPageURI     types.NullString
	previousPageURI types.NullString
}

func (p *IncomingNumberPage) Numbers() []*IncomingNumber {
	return p.numbers
}

func (p *IncomingNumberPage) NextPageURI() types.NullString {
	return p.nextPageURI
}

func (p *IncomingNumberPage) PreviousPageURI() types.NullString {
	return p.previousPageURI
}

type IncomingNumber struct {
	user   *config.User
	number *twilio.IncomingPhoneNumber
}

func NewIncomingNumber(pn *twilio.IncomingPhoneNumber, p *config.Permission, u *config.User) (*IncomingNumber, error) {
	if pn.DateCreated.Valid == false {
		return nil, errors.New("Invalid DateCreated for phone number")
	}
	if !u.CanViewResource(pn.DateCreated.Time, p.MaxResourceAge()) {
		return nil, config.ErrTooOld
	}
	return &IncomingNumber{user: u, number: pn}, nil
}

func NewIncomingNumberPage(pn *twilio.IncomingPhoneNumberPage, p *config.Permission, u *config.User) (*IncomingNumberPage, error) {
	numbers := make([]*IncomingNumber, 0)
	for _, number := range pn.IncomingPhoneNumbers {
		num, err := NewIncomingNumber(number, p, u)
		if err == config.ErrTooOld || err == config.PermissionDenied {
			continue
		}
		if err != nil {
			return nil, err
		}
		numbers = append(numbers, num)
	}
	var npuri types.NullString
	if len(numbers) > 0 {
		npuri = pn.NextPageURI
	}
	return &IncomingNumberPage{
		numbers:         numbers,
		nextPageURI:     npuri,
		previousPageURI: pn.PreviousPageURI,
	}, nil
}

func (n *IncomingNumber) CanViewProperty(property string) bool {
	if n.number == nil {
		return false
	}
	switch property {
	case "Sid", "DateCreated", "PhoneNumber", "FriendlyName":
		return true
	default:
		panic("unknown property " + property)
	}
}

func (n *IncomingNumber) Sid() (string, error) {
	if n.CanViewProperty("Sid") {
		return n.number.Sid, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (a *IncomingNumber) DateCreated() (twilio.TwilioTime, error) {
	if a.CanViewProperty("DateCreated") {
		return a.number.DateCreated, nil
	} else {
		return twilio.TwilioTime{}, config.PermissionDenied
	}
}

func (n *IncomingNumber) PhoneNumber() (twilio.PhoneNumber, error) {
	if n.CanViewProperty("PhoneNumber") {
		return n.number.PhoneNumber, nil
	} else {
		return twilio.PhoneNumber(""), config.PermissionDenied
	}
}

func (n *IncomingNumber) FriendlyName() (string, error) {
	if n.CanViewProperty("FriendlyName") {
		return n.number.FriendlyName, nil
	} else {
		return "", config.PermissionDenied
	}
}
