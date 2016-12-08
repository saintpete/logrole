package views

import (
	"errors"

	types "github.com/kevinburke/go-types"
	twilio "github.com/saintpete/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
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
	// The recording URL, encrypted with the secret key. This must be set in
	// NewRecording.
	url string
}

func (r *Recording) CanViewProperty(property string) bool {
	switch property {
	case "Sid", "DateCreated", "DateUpdated", "Duration":
		return r.user.CanPlayRecordings()
	case "Price", "PriceUnit":
		return r.user.CanViewRecordingPrice()
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

func (r *Recording) Price() (string, error) {
	if r.CanViewProperty("Price") {
		return r.recording.Price, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (r *Recording) PriceUnit() (string, error) {
	if r.CanViewProperty("PriceUnit") {
		return r.recording.PriceUnit, nil
	} else {
		return "", config.PermissionDenied
	}
}

func (r *Recording) FriendlyPrice() (string, error) {
	if r.CanViewProperty("Price") && r.CanViewProperty("PriceUnit") {
		return r.recording.FriendlyPrice(), nil
	} else {
		return "", config.PermissionDenied
	}
}

func (r *Recording) CanPlay() bool {
	return r.user.CanPlayRecordings()
}

// URL returns the encrypted URL of the recording.
func (r *Recording) URL() (string, error) {
	if r.user.CanPlayRecordings() {
		return r.url, nil
	} else {
		return "", config.PermissionDenied
	}
}

const defaultMediaType = "audio/x-wav"

// MediaType returns the Content-Type for the encrypted recording's URL.
func (r *Recording) MediaType() string {
	return defaultMediaType
}

func NewRecording(r *twilio.Recording, p *config.Permission, u *config.User, key *[32]byte) (*Recording, error) {
	if r.DateCreated.Valid == false {
		return nil, errors.New("Invalid DateCreated for recording")
	}
	if !u.CanViewResource(r.DateCreated.Time, p.MaxResourceAge()) {
		return nil, config.ErrTooOld
	}
	url := services.Opaque(r.URL(".wav"), key)
	return &Recording{
		user:      u,
		recording: r,
		url:       "/audio/" + url,
	}, nil
}

func NewRecordingPage(rp *twilio.RecordingPage, p *config.Permission, u *config.User, key *[32]byte) (*RecordingPage, error) {
	recordings := make([]*Recording, 0)
	for _, trecording := range rp.Recordings {
		recording, err := NewRecording(trecording, p, u, key)
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
