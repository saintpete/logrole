package views

import (
	"errors"

	types "github.com/kevinburke/go-types"
	twilio "github.com/saintpete/twilio-go"
	"github.com/saintpete/logrole/config"
)

type CallPage struct {
	calls           []*Call
	previousPageURI types.NullString
	nextPageURI     types.NullString
}

type Call struct {
	user *config.User
	call *twilio.Call
}

func NewCall(call *twilio.Call, p *config.Permission, u *config.User) (*Call, error) {
	if u.CanViewCalls() == false {
		return nil, config.PermissionDenied
	}
	if call.DateCreated.Valid == false {
		return nil, errors.New("Invalid DateCreated for call")
	}
	if !u.CanViewResource(call.DateCreated.Time, p.MaxResourceAge()) {
		return nil, config.ErrTooOld
	}
	return &Call{user: u, call: call}, nil
}

func (c *Call) CanViewProperty(property string) bool {
	if c.user == nil {
		return false
	}
	switch property {
	case "Sid", "Direction", "Status", "DateCreated", "DateUpdated",
		"Duration", "StartTime", "EndTime":
		return c.user.CanViewCalls()
	case "Price", "PriceUnit":
		return c.user.CanViewCallPrice()
	case "From":
		return c.user.CanViewCallFrom()
	case "To":
		return c.user.CanViewCallTo()
	default:
		panic("unknown property " + property)
	}
}

func (c *Call) Sid() (string, error) {
	if c.CanViewProperty("Sid") {
		return c.call.Sid, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (c *Call) DateCreated() (twilio.TwilioTime, error) {
	if c.CanViewProperty("DateCreated") {
		return c.call.DateCreated, nil
	} else {
		return twilio.TwilioTime{}, config.PermissionDenied
	}
}

func (c *Call) Direction() (twilio.Direction, error) {
	if c.CanViewProperty("Direction") {
		return c.call.Direction, nil
	} else {
		return twilio.Direction(""), config.PermissionDenied
	}
}

func (c *Call) Status() (twilio.Status, error) {
	if c.CanViewProperty("Status") {
		return c.call.Status, nil
	} else {
		return twilio.Status(""), config.PermissionDenied
	}
}

func (c *Call) From() (twilio.PhoneNumber, error) {
	if c.CanViewProperty("From") {
		return c.call.From, nil
	} else {
		return twilio.PhoneNumber(""), config.PermissionDenied
	}
}

func (c *Call) To() (twilio.PhoneNumber, error) {
	if c.CanViewProperty("To") {
		return c.call.To, nil
	} else {
		return twilio.PhoneNumber(""), config.PermissionDenied
	}
}

func (c *Call) Duration() (twilio.TwilioDuration, error) {
	if c.CanViewProperty("Duration") {
		return c.call.Duration, nil
	} else {
		return twilio.TwilioDuration(0), config.PermissionDenied
	}
}

func (c *Call) StartTime() (twilio.TwilioTime, error) {
	if c.CanViewProperty("StartTime") {
		return c.call.StartTime, nil
	} else {
		return twilio.TwilioTime{}, config.PermissionDenied
	}
}

func (c *Call) FriendlyPrice() (string, error) {
	if c.CanViewProperty("Price") && c.CanViewProperty("PriceUnit") {
		return c.call.FriendlyPrice(), nil
	} else {
		return "", config.PermissionDenied
	}
}

func (c *Call) CanViewNumRecordings() bool {
	return c.user.CanViewNumRecordings()
}

func (c *Call) CanViewCallAlerts() bool {
	return c.user.CanViewAlerts()
}

func (c *Call) Failed() (bool, error) {
	if c.CanViewProperty("Status") {
		return c.call.Status == twilio.StatusFailed, nil
	} else {
		return false, config.PermissionDenied
	}
}

func NewCallPage(cp *twilio.CallPage, p *config.Permission, u *config.User) (*CallPage, error) {
	if u.CanViewCalls() == false {
		return nil, config.PermissionDenied
	}
	calls := make([]*Call, 0)
	for _, call := range cp.Calls {
		cl, err := NewCall(call, p, u)
		if err == config.ErrTooOld || err == config.PermissionDenied {
			continue
		}
		if err != nil {
			return nil, err
		}
		calls = append(calls, cl)
	}
	var npuri types.NullString
	if len(calls) > 0 {
		npuri = cp.NextPageURI
	}
	return &CallPage{
		calls:           calls,
		nextPageURI:     npuri,
		previousPageURI: cp.PreviousPageURI,
	}, nil
}

func (cp *CallPage) Calls() []*Call {
	return cp.calls
}

func (cp *CallPage) NextPageURI() types.NullString {
	return cp.nextPageURI
}

func (cp *CallPage) PreviousPageURI() types.NullString {
	return cp.previousPageURI
}

// ShowHeader returns true if we should show the table header in the call
// list view. This is true if the user is allowed to view the fieldName on any
// message in the list, and true if there are no messages.
func (cp *CallPage) ShowHeader(fieldName string) bool {
	if cp == nil {
		return true
	}
	calls := cp.Calls()
	if len(calls) == 0 {
		return showAllColumnsOnEmptyPage
	}
	for _, call := range calls {
		if call.CanViewProperty(fieldName) {
			return true
		}
	}
	return false
}
