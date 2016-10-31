package views

import (
	"errors"
	"time"

	types "github.com/kevinburke/go-types"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
)

type AlertPage struct {
	alerts      []*Alert
	nextPageURI types.NullString
}

type Alert struct {
	user  *config.User
	alert *twilio.Alert
}

func NewAlert(alert *twilio.Alert, p *config.Permission, u *config.User) (*Alert, error) {
	if alert.DateCreated.Valid == false {
		return nil, errors.New("Invalid DateCreated for alert")
	}
	oldest := time.Now().UTC().Add(-1 * p.MaxResourceAge())
	if alert.DateCreated.Time.Before(oldest) {
		return nil, config.ErrTooOld
	}
	return &Alert{user: u, alert: alert}, nil
}

func NewAlertPage(cp *twilio.AlertPage, p *config.Permission, u *config.User) (*AlertPage, error) {
	alerts := make([]*Alert, 0)
	for _, alert := range cp.Alerts {
		cl, err := NewAlert(alert, p, u)
		if err == config.ErrTooOld || err == config.PermissionDenied {
			continue
		}
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, cl)
	}
	var npuri types.NullString
	if len(alerts) > 0 {
		npuri = cp.Meta.NextPageURL
	}
	return &AlertPage{
		alerts:      alerts,
		nextPageURI: npuri,
	}, nil
}

func (a *AlertPage) Alerts() []*Alert {
	return a.alerts
}

func (c *Alert) CanViewProperty(property string) bool {
	if c.user == nil {
		return false
	}
	switch property {
	case "Sid", "ErrorCode", "RequestURL", "RequestMethod":
		return c.user.CanViewCallAlerts()
	default:
		panic("unknown property " + property)
	}
}

func (a *Alert) Sid() (string, error) {
	if a.CanViewProperty("Sid") {
		return a.alert.Sid, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (a *Alert) ErrorCode() (twilio.Code, error) {
	if a.CanViewProperty("ErrorCode") {
		return a.alert.ErrorCode, nil
	} else {
		return twilio.Code(0), config.PermissionDenied
	}
}

func (a *Alert) RequestMethod() (string, error) {
	if a.CanViewProperty("RequestMethod") {
		return a.alert.RequestMethod, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (a *Alert) RequestURL() (string, error) {
	if a.CanViewProperty("RequestURL") {
		return a.alert.RequestURL, nil
	} else {
		return "", config.PermissionDenied
	}
}
