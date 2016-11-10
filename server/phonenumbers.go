package server

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	types "github.com/kevinburke/go-types"
	"github.com/kevinburke/rest"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
	"github.com/saintpete/logrole/views"
)

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
	return []string{"phone-number", "next"}
}

func (s *numberListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &baseData{
		LF: s.LocationFinder,
		Data: &alertListData{
			Err:   str,
			Loc:   s.LocationFinder.GetLocationReq(r),
			Query: query,
			Page:  new(views.AlertPage),
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
	page := new(views.IncomingNumberPage)
	cachedAt := time.Time{}
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
		CachedAt: cachedAt,
		Data: &numberListData{
			Page:                  page,
			Query:                 query,
			Loc:                   loc,
			EncryptedNextPage:     getEncryptedPage(page.NextPageURI(), s.secretKey),
			EncryptedPreviousPage: getEncryptedPage(page.PreviousPageURI(), s.secretKey),
		}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}
