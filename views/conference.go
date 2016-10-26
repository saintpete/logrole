package views

import (
	"errors"
	"time"

	types "github.com/kevinburke/go-types"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
)

type Conference struct {
	user       *config.User
	conference *twilio.Conference
}

type ConferencePage struct {
	conferences     []*Conference
	previousPageURI types.NullString
	nextPageURI     types.NullString
}

func (c *ConferencePage) Conferences() []*Conference {
	return c.conferences
}

func (cp *ConferencePage) NextPageURI() types.NullString {
	return cp.nextPageURI
}

func (cp *ConferencePage) PreviousPageURI() types.NullString {
	return cp.previousPageURI
}

func (cp *ConferencePage) ShowHeader(fieldName string) bool {
	if cp == nil {
		return showAllColumnsOnEmptyPage
	}
	conferences := cp.Conferences()
	if len(conferences) == 0 {
		return showAllColumnsOnEmptyPage
	}
	for _, conference := range conferences {
		if conference.CanViewProperty(fieldName) {
			return true
		}
	}
	return false
}

func NewConference(conference *twilio.Conference, p *config.Permission, u *config.User) (*Conference, error) {
	if conference.DateCreated.Valid == false {
		return nil, errors.New("Invalid DateCreated for conference")
	}
	oldest := time.Now().UTC().Add(-1 * p.MaxResourceAge())
	if conference.DateCreated.Time.Before(oldest) {
		return nil, config.ErrTooOld
	}
	return &Conference{user: u, conference: conference}, nil
}

func NewConferencePage(mp *twilio.ConferencePage, p *config.Permission, u *config.User) (*ConferencePage, error) {
	conferences := make([]*Conference, 0)
	for _, conference := range mp.Conferences {
		conference, err := NewConference(conference, p, u)
		if err == config.ErrTooOld || err == config.PermissionDenied {
			continue
		}
		if err != nil {
			return nil, err
		}
		conferences = append(conferences, conference)
	}
	var npuri types.NullString
	if len(conferences) > 0 {
		npuri = mp.NextPageURI
	}
	return &ConferencePage{
		conferences:     conferences,
		nextPageURI:     npuri,
		previousPageURI: mp.PreviousPageURI,
	}, nil
}

func (c *Conference) CanViewProperty(property string) bool {
	if c.user == nil {
		return false
	}
	switch property {
	case "Sid", "DateCreated", "DateUpdated", "APIVersion", "AccountSID",
		"URI", "Status", "FriendlyName", "Region":
		return c.user.CanViewConferences()
	default:
		panic("unknown property " + property)
	}
}

func (c *Conference) FriendlyName() (string, error) {
	if c.user.CanViewConferences() {
		return c.conference.FriendlyName, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (c *Conference) Sid() (string, error) {
	if c.user.CanViewConferences() {
		return c.conference.Sid, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (c *Conference) Region() (string, error) {
	if c.user.CanViewConferences() {
		return c.conference.Region, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (c *Conference) Status() (twilio.Status, error) {
	if c.user.CanViewConferences() {
		return c.conference.Status, nil
	} else {
		return twilio.Status(""), config.PermissionDenied
	}
}

func (c *Conference) DateCreated() (twilio.TwilioTime, error) {
	if c.user.CanViewConferences() {
		return c.conference.DateCreated, nil
	} else {
		return twilio.TwilioTime{}, config.PermissionDenied
	}
}
