package views

import (
	"errors"
	"time"

	types "github.com/kevinburke/go-types"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
)

type RecordingPage struct {
	recordings  []*Recording
	nextPageURI types.NullString
}

func (r *RecordingPage) Recordings() []*Recording {
	return r.recordings
}

func (r *RecordingPage) NextPageURI() types.NullString {
	return r.nextPageURI
}

type Recording struct {
	user      *config.User
	recording *twilio.Recording
}

func (r *Recording) CanViewProperty(property string) bool {
	switch property {
	case "Sid", "DateCreated", "DateUpdated", "Duration":
		return r.user.CanPlayRecordings()
	default:
		panic("Unknown property " + property)
	}
}

func (r *Recording) Sid() (string, error) {
	if r.CanViewProperty("Sid") {
		return r.recording.Sid, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (r *Recording) Duration() (twilio.TwilioDuration, error) {
	if r.CanViewProperty("Duration") {
		return r.recording.Duration, nil
	} else {
		return twilio.TwilioDuration(0), config.PermissionDenied
	}
}

func (r *Recording) DateCreated() (twilio.TwilioTime, error) {
	if r.CanViewProperty("DateCreated") {
		return r.recording.DateCreated, nil
	} else {
		return twilio.TwilioTime{}, config.PermissionDenied
	}
}

func (r *Recording) CanPlay() bool {
	return r.user.CanPlayRecordings()
}

func (r *Recording) URL(extension string) (string, error) {
	if r.user.CanPlayRecordings() {
		return r.recording.URL(extension), nil
	} else {
		return "", config.PermissionDenied
	}
}

func NewRecording(r *twilio.Recording, p *config.Permission, u *config.User) (*Recording, error) {
	if r.DateCreated.Valid == false {
		return nil, errors.New("Invalid DateCreated for recording")
	}
	oldest := time.Now().UTC().Add(-1 * p.MaxResourceAge())
	if r.DateCreated.Time.Before(oldest) {
		return nil, config.ErrTooOld
	}
	return &Recording{user: u, recording: r}, nil
}

func NewRecordingPage(rp *twilio.RecordingPage, p *config.Permission, u *config.User) (*RecordingPage, error) {
	recordings := make([]*Recording, 0)
	for _, trecording := range rp.Recordings {
		recording, err := NewRecording(trecording, p, u)
		if err == config.ErrTooOld || err == config.PermissionDenied {
			continue
		}
		if err != nil {
			return nil, err
		}
		recordings = append(recordings, recording)
	}
	return &RecordingPage{recordings: recordings, nextPageURI: rp.NextPageURI}, nil
}
