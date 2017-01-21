package server

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aristanetworks/goarista/monotime"
	log "github.com/inconshreveable/log15"
	types "github.com/kevinburke/go-types"
	"github.com/kevinburke/rest"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
	twilio "github.com/saintpete/twilio-go"
)

// We handle two different routes - /phone-numbers/PN123 and
// /phone-numbers/+1925... The former redirects to the latter
const numberInstancePattern = `(?P<number>[^/\s]+)`

// for PN123 redirects
const numberSidPattern = `(?P<sid>PN[a-f0-9]{32})`

var numberInstanceRoute = regexp.MustCompile("^/phone-numbers/" + numberInstancePattern + "$")
var numberSidRegex = regexp.MustCompile(numberSidPattern)

type numberListServer struct {
	log.Logger
	Client         views.Client
	PageSize       uint
	MaxResourceAge time.Duration
	LocationFinder services.LocationFinder
	secretKey      *[32]byte
	tpl            *template.Template
}

func newNumberListServer(l log.Logger, vc views.Client,
	lf services.LocationFinder, pageSize uint, maxResourceAge time.Duration,
	secretKey *[32]byte) (*numberListServer, error) {
	s := &numberListServer{
		Logger:         l,
		Client:         vc,
		PageSize:       pageSize,
		LocationFinder: lf,
		MaxResourceAge: maxResourceAge,
		secretKey:      secretKey,
	}
	tpl, err := newTpl(template.FuncMap{}, base+numberListTpl+pagingTpl)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
	return s, nil
}

func (s *numberListServer) validParams() []string {
	return []string{"phone-number", "friendly-name", "next"}
}

func (s *numberListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	str := cleanError(err)
	data := &baseData{
		LF: s.LocationFinder,
		Data: &numberListData{
			Err:   str,
			Loc:   s.LocationFinder.GetLocationReq(r),
			Query: query,
			Page:  new(views.IncomingNumberPage),
		},
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
		return
	}
}

type numberListData struct {
	Page                  *views.IncomingNumberPage
	EncryptedNextPage     string
	EncryptedPreviousPage string
	Loc                   *time.Location
	Err                   string
	Query                 url.Values
}

func (d *numberListData) Title() string {
	return "Phone Numbers"
}

func (ad *numberListData) Path() string {
	return "/phone-numbers"
}

func (c *numberListData) NextQuery() template.URL {
	data := url.Values{}
	if c.EncryptedNextPage != "" {
		data.Set("next", c.EncryptedNextPage)
	}
	return template.URL(data.Encode())
}

func (c *numberListData) PreviousQuery() template.URL {
	data := url.Values{}
	if c.EncryptedPreviousPage != "" {
		data.Set("next", c.EncryptedPreviousPage)
	}
	return template.URL(data.Encode())
}

func (s *numberListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	query := r.URL.Query()
	if err := validateParams(s.validParams(), query); err != nil {
		s.renderError(w, r, http.StatusBadRequest, query, err)
		return
	}
	loc := s.LocationFinder.GetLocationReq(r)
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	var err error
	next, nextErr := getNext(query, s.secretKey)
	if nextErr != nil {
		err = errors.New("Could not decrypt `next` query parameter: " + nextErr.Error())
		s.renderError(w, r, http.StatusBadRequest, query, err)
		return
	}
	var page *views.IncomingNumberPage
	var cachedAt uint64
	start := monotime.Now()
	if next != "" {
		if !strings.HasPrefix(next, "/"+twilio.APIVersion) {
			s.Warn("Invalid next page URI", "next", next, "opaque", query.Get("next"))
			s.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, cachedAt, err = s.Client.GetNextNumberPage(ctx, u, next)
		setNextPageValsOnQuery(next, query)
	} else {
		vals := url.Values{}
		vals.Set("PageSize", strconv.FormatUint(uint64(s.PageSize), 10))
		if filterErr := setPageFilters(query, vals); filterErr != nil {
			s.renderError(w, r, http.StatusBadRequest, query, filterErr)
			return
		}
		page, cachedAt, err = s.Client.GetNumberPage(ctx, u, vals)
	}
	if err == twilio.NoMoreResults {
		page = new(views.IncomingNumberPage)
		err = nil
	}
	if err != nil {
		switch terr := err.(type) {
		case *rest.Error:
			switch terr.StatusCode {
			case 400:
				s.renderError(w, r, http.StatusBadRequest, query, err)
			case 404:
				rest.NotFound(w, r)
			default:
				rest.ServerError(w, r, terr)
			}
		default:
			rest.ServerError(w, r, err)
		}
		return
	}
	go func(u *config.User, n types.NullString) {
		if n.Valid {
			if _, _, err := s.Client.GetNextNumberPage(context.Background(), u, n.String); err != nil {
				s.Debug("Error fetching next page", "err", err)
			}
		}
	}(u, page.NextPageURI())
	data := &baseData{
		LF:       s.LocationFinder,
		Duration: monotime.Since(start),
		Data: &numberListData{
			Page:                  page,
			Query:                 query,
			Loc:                   loc,
			EncryptedNextPage:     getEncryptedPage(page.NextPageURI(), s.secretKey),
			EncryptedPreviousPage: getEncryptedPage(page.PreviousPageURI(), s.secretKey),
		}}
	if cachedAt > 0 {
		data.CachedDuration = monotime.Since(cachedAt)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}

type numberInstanceServer struct {
	log.Logger
	Client         views.Client
	LocationFinder services.LocationFinder
	tpl            *template.Template
}

func newNumberInstanceServer(l log.Logger, vc views.Client, lf services.LocationFinder) (*numberInstanceServer, error) {
	s := &numberInstanceServer{
		Logger:         l,
		Client:         vc,
		LocationFinder: lf,
	}
	tpl, err := newTpl(template.FuncMap{
		"is_our_pn": vc.IsTwilioNumber,
	}, base+messageStatusTpl+messageSummaryTpl+callSummaryTpl+phoneTpl+
		numberInstanceTpl+sidTpl+copyScript)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
	return s, nil
}

// ugh, go templates
type msgPageLoc struct {
	Page *views.MessagePage
	// False for "Messages to this number"
	IsFrom bool
	Loc    *time.Location
	Number string
}

type callPageLoc struct {
	Page *views.CallPage
	// False for "Calls to this number"
	IsFrom bool
	Loc    *time.Location
	Number string
}

type numberInstanceData struct {
	Number       *views.IncomingNumber
	OwnNumber    bool
	Loc          *time.Location
	SMSFrom      *msgPageLoc
	SMSFromErr   string
	SMSTo        *msgPageLoc
	SMSToErr     string
	CallsFrom    *callPageLoc
	CallsFromErr string
	CallsTo      *callPageLoc
	CallsToErr   string
}

func (n *numberInstanceData) Title() string {
	if n != nil && n.Number != nil && n.Number.CanViewProperty("PhoneNumber") {
		num, _ := n.Number.PhoneNumber()
		return "Number " + num.Friendly()
	}
	return "Number Details"
}

// Given a PN sid, retrieve it from the API to get the associated phone number,
// then redirect to the phone-number URL.
func (s *numberInstanceServer) redirectPN(w http.ResponseWriter, r *http.Request, u *config.User, pnSid string) {
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	number, err := s.Client.GetIncomingNumber(ctx, u, pnSid)
	switch err {
	case nil:
		break
	case config.PermissionDenied, config.ErrTooOld:
		rest.Forbidden(w, r, &rest.Error{Title: err.Error()})
		return
	default:
		switch terr := err.(type) {
		case *rest.Error:
			switch terr.StatusCode {
			case 404:
				rest.NotFound(w, r)
			default:
				rest.ServerError(w, r, terr)
			}
		default:
			rest.ServerError(w, r, err)
		}
		return
	}
	pn, err := number.PhoneNumber()
	if err != nil {
		rest.Forbidden(w, r, &rest.Error{Title: err.Error()})
		return
	}
	http.Redirect(w, r, "/phone-numbers/"+string(pn), 301)
	return
}

func (s *numberInstanceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if match := numberSidRegex.FindStringSubmatch(r.URL.Path); len(match) > 0 {
		s.redirectPN(w, r, u, match[0])
		return
	}
	pn := numberInstanceRoute.FindStringSubmatch(r.URL.Path)[1]
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(4)
	loc := s.LocationFinder.GetLocationReq(r)
	innerData := &numberInstanceData{
		Loc: loc,
	}
	start := monotime.Now()
	number, err := s.Client.GetIncomingNumberByPN(ctx, u, pn)
	go func() {
		// get SMS from this number
		data := url.Values{}
		data.Set("From", pn)
		data.Set("PageSize", "20")
		fromMsgs, _, err := s.Client.GetMessagePageInRange(ctx, u, twilio.Epoch, twilio.HeatDeath, data)
		if err == nil || err == twilio.NoMoreResults {
			innerData.SMSFrom = &msgPageLoc{
				Page:   fromMsgs,
				IsFrom: true,
				Loc:    loc,
				Number: pn,
			}
		} else {
			innerData.SMSFromErr = err.Error()
		}
		wg.Done()
	}()
	go func() {
		// get SMS to this number
		data := url.Values{}
		data.Set("To", pn)
		data.Set("PageSize", "20")
		toMsgs, _, err := s.Client.GetMessagePageInRange(ctx, u, twilio.Epoch, twilio.HeatDeath, data)
		if err == nil || err == twilio.NoMoreResults {
			innerData.SMSTo = &msgPageLoc{
				Page:   toMsgs,
				IsFrom: false,
				Loc:    loc,
				Number: pn,
			}
		} else {
			innerData.SMSToErr = err.Error()
		}
		wg.Done()
	}()
	go func() {
		// get Calls to this number
		data := url.Values{}
		data.Set("To", pn)
		data.Set("PageSize", "20")
		callsTo, _, err := s.Client.GetCallPageInRange(ctx, u, twilio.Epoch, twilio.HeatDeath, data)
		if err == nil || err == twilio.NoMoreResults {
			innerData.CallsTo = &callPageLoc{
				Page:   callsTo,
				IsFrom: false,
				Loc:    loc,
				Number: pn,
			}
		} else {
			innerData.CallsToErr = err.Error()
		}
		wg.Done()
	}()
	go func() {
		// get Calls from this number
		data := url.Values{}
		data.Set("From", pn)
		data.Set("PageSize", "20")
		callsFrom, _, err := s.Client.GetCallPageInRange(ctx, u, twilio.Epoch, twilio.HeatDeath, data)
		if err == nil || err == twilio.NoMoreResults {
			innerData.CallsFrom = &callPageLoc{
				Page:   callsFrom,
				IsFrom: false,
				Loc:    loc,
				Number: pn,
			}
		} else {
			innerData.CallsFromErr = err.Error()
		}
		wg.Done()
	}()
	switch err {
	case nil:
		break
	case config.PermissionDenied, config.ErrTooOld:
		rest.Forbidden(w, r, &rest.Error{Title: err.Error()})
		return
	default:
		switch terr := err.(type) {
		case *rest.Error:
			switch terr.StatusCode {
			case 404:
				// We still want to show the calls to/from this number
				innerData.OwnNumber = false
				break
			default:
				rest.ServerError(w, r, terr)
				return
			}
		default:
			rest.ServerError(w, r, err)
			return
		}
	}
	wg.Wait()
	innerData.Number = number
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	data := &baseData{
		LF:       s.LocationFinder,
		Duration: monotime.Since(start),
		Data:     innerData,
	}
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}
