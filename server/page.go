package server

import (
	"net/url"

	types "github.com/kevinburke/go-types"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/services"
)

// Code that's shared across list views

func getEncryptedNextPage(npuri types.NullString, secretKey *[32]byte) (string, error) {
	if !npuri.Valid {
		return "", nil
	}
	return services.Opaque(npuri.String, secretKey)
}

// setNextPageValsOnQuery takes query values that have been sent to the Twilio
// API, and sets them on the provided query object. We use this to populate the
// search fields on the message/call search pages.
func setNextPageValsOnQuery(nextpageuri string, query url.Values) {
	u, err := url.Parse(nextpageuri)
	if err != nil {
		return
	}
	nq := u.Query()
	if start := nq.Get("DateSent>"); start != "" {
		query.Set("start", start)
	}
	if start := nq.Get("StartTime>"); start != "" {
		query.Set("start-after", start)
	}
	if start := nq.Get("StartTime<"); start != "" {
		query.Set("start-before", start)
	}
	if end := nq.Get("DateSent<"); end != "" {
		query.Set("end", end)
	}
	if from := nq.Get("From"); from != "" {
		query.Set("from", from)
	}
	if to := nq.Get("To"); to != "" {
		query.Set("to", to)
	}
}

// Reverse of the function above, with validation
func setPageFilters(query url.Values, pageFilters url.Values) error {
	if from := query.Get("from"); from != "" {
		fromPN, err := twilio.NewPhoneNumber(from)
		if err != nil {
			query.Del("from")
			return err
		}
		s := string(fromPN)
		pageFilters.Set("From", s)
		query.Set("from", s)
	}
	if to := query.Get("to"); to != "" {
		toPN, err := twilio.NewPhoneNumber(to)
		if err != nil {
			query.Del("to")
			return err
		}
		s := string(toPN)
		pageFilters.Set("To", s)
		query.Set("to", s)
	}
	// NB - we purposely don't do date validation here since we filter out
	// older messages as part of the message view.
	if startDate := query.Get("start"); startDate != "" {
		pageFilters.Set("DateSent>", startDate)
	}
	if end := query.Get("end"); end != "" {
		pageFilters.Set("DateSent<", end)
	}
	if startDate := query.Get("start-after"); startDate != "" {
		pageFilters.Set("StartTime>", startDate)
	}
	if startDate := query.Get("start-before"); startDate != "" {
		pageFilters.Set("StartTime<", startDate)
	}
	return nil
}
