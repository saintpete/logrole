package views

import (
	"errors"

	types "github.com/kevinburke/go-types"
	twilio "github.com/saintpete/twilio-go"
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
	// NB: Phone numbers are *exempt* from max resource age rules, they don't
	// really make sense.
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

func (page *IncomingNumberPage) ShowHeader(property string) bool {
	if page == nil {
		return showAllColumnsOnEmptyPage
	}
	numbers := page.Numbers()
	if len(numbers) == 0 {
		return showAllColumnsOnEmptyPage
	}
	for _, number := range numbers {
		var show bool
		switch property {
		default:
			show = number.CanViewProperty(property)
		}
		if show {
			return true
		}
	}
	return false
}

func (n *IncomingNumber) CanViewProperty(property string) bool {
	if n.number == nil {
		return false
	}
	switch property {
	case "Sid", "DateCreated", "PhoneNumber", "FriendlyName", "Beta",
		"TrunkSid", "Capabilities", "EmergencyStatus":
		return true
	case "VoiceURL", "SMSURL", "VoiceMethod", "SMSMethod", "StatusCallback",
		"StatusCallbackMethod", "VoiceFallbackURL", "VoiceFallbackMethod",
		"SMSFallbackURL", "SMSFallbackMethod", "VoiceApplicationSid",
		"SMSApplicationSid":
		return n.user.CanViewCallbackURLs()
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

func (n *IncomingNumber) Capabilities() (*twilio.NumberCapability, error) {
	if n.CanViewProperty("Capabilities") {
		return n.number.Capabilities, nil
	} else {
		return nil, config.PermissionDenied
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

func (n *IncomingNumber) VoiceURL() (string, error) {
	if n.CanViewProperty("VoiceURL") {
		return n.number.VoiceURL, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) VoiceMethod() (string, error) {
	if n.CanViewProperty("VoiceMethod") {
		return n.number.VoiceMethod, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) SMSURL() (string, error) {
	if n.CanViewProperty("SMSURL") {
		return n.number.SMSURL, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) Beta() (bool, error) {
	if n.CanViewProperty("Beta") {
		return n.number.Beta, nil
	} else {
		return false, config.PermissionDenied
	}
}

func (n *IncomingNumber) SMSMethod() (string, error) {
	if n.CanViewProperty("SMSMethod") {
		return n.number.SMSMethod, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) StatusCallback() (string, error) {
	if n.CanViewProperty("StatusCallback") {
		return n.number.StatusCallback, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) StatusCallbackMethod() (string, error) {
	if n.CanViewProperty("StatusCallbackMethod") {
		return n.number.StatusCallbackMethod, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) VoiceFallbackMethod() (string, error) {
	if n.CanViewProperty("VoiceFallbackMethod") {
		return n.number.VoiceFallbackMethod, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) VoiceFallbackURL() (string, error) {
	if n.CanViewProperty("VoiceFallbackURL") {
		return n.number.VoiceFallbackURL, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) SMSFallbackMethod() (string, error) {
	if n.CanViewProperty("SMSFallbackMethod") {
		return n.number.SMSFallbackMethod, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) SMSFallbackURL() (string, error) {
	if n.CanViewProperty("SMSFallbackURL") {
		return n.number.SMSFallbackURL, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) VoiceApplicationSid() (string, error) {
	if n.CanViewProperty("VoiceApplicationSid") {
		return n.number.VoiceApplicationSid, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) SMSApplicationSid() (string, error) {
	if n.CanViewProperty("SMSApplicationSid") {
		return n.number.SMSApplicationSid, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) EmergencyStatus() (string, error) {
	if n.CanViewProperty("EmergencyStatus") {
		return n.number.EmergencyStatus, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (n *IncomingNumber) TrunkSid() (types.NullString, error) {
	if n.CanViewProperty("TrunkSid") {
		return n.number.TrunkSid, nil
	} else {
		return types.NullString{}, config.PermissionDenied
	}
}
