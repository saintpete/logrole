package server

import (
	"net/http"
	"net/url"
	"time"

	types "github.com/kevinburke/go-types"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/services"
)

const HTML5DatetimeLocalFormat = "2006-01-02T15:04"

// Code that's shared across list views

func getEncryptedPage(npuri types.NullString, secretKey *[32]byte) string {
	if !npuri.Valid {
		return ""
	}
	return services.Opaque(npuri.String, secretKey)
}

func getNext(query url.Values, secretKey *[32]byte) (string, error) {
	if query == nil {
		return "", nil
	}
	if opaqueNext := query.Get("next"); opaqueNext == "" {
		return "", nil
	} else {
		return services.Unopaque(opaqueNext, secretKey)
	}
}

type errorRenderer interface {
	renderError(http.ResponseWriter, *http.Request, int, url.Values, error)
}

func getTimes(w http.ResponseWriter, r *http.Request, startVal, endVal string, loc *time.Location, query url.Values, renderer errorRenderer) (time.Time, time.Time, bool) {
	var startTime, endTime time.Time
	var err error
	start := query.Get(startVal)
	if start == "" {
		startTime = twilio.Epoch
	} else {
		startTime, err = time.ParseInLocation(HTML5DatetimeLocalFormat, start, loc)
		if err != nil {
			renderer.renderError(w, r, http.StatusBadRequest, query, err)
			return startTime, endTime, true
		}
		startTime = startTime.In(loc)
	}
	end := query.Get(endVal)
	if end == "" {
		endTime = twilio.HeatDeath
	} else {
		endTime, err = time.ParseInLocation(HTML5DatetimeLocalFormat, end, loc)
		if err != nil {
			renderer.renderError(w, r, http.StatusBadRequest, query, err)
			return startTime, endTime, true
		}
		endTime = endTime.In(loc)
	}
	return startTime, endTime, false
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
	if from := nq.Get("From"); from != "" {
		query.Set("from", from)
	}
	if to := nq.Get("To"); to != "" {
		query.Set("to", to)
	}
}

// Reverse of the function above, with validation. Every list filter calls this
// function to set Twilio search filters, so the query keys should be unique.
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
	// conferences
	if friendlyName := query.Get("friendly-name"); friendlyName != "" {
		pageFilters.Set("FriendlyName", friendlyName)
	}
	if status := query.Get("status"); status != "" {
		pageFilters.Set("Status", status)
	}
	return nil
}
