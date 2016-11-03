package server

import (
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
	"golang.org/x/net/context"
)

type alertListServer struct {
	log.Logger
	Client         views.Client
	PageSize       uint
	MaxResourceAge time.Duration
	LocationFinder services.LocationFinder
	secretKey      *[32]byte
	tpl            *template.Template
}

type alertListData struct {
	Page                  *views.AlertPage
	EncryptedNextPage     string
	EncryptedPreviousPage string
	Loc                   *time.Location
	Query                 url.Values
	Err                   string
}

func (ad *alertListData) Title() string {
	return "Alerts"
}

func (ad *alertListData) Path() string {
	return "/alerts"
}

func newAlertListServer(l log.Logger, vc views.Client,
	lf services.LocationFinder, pageSize uint, maxResourceAge time.Duration,
	secretKey *[32]byte) (*alertListServer, error) {
	s := &alertListServer{
		Logger:         l,
		Client:         vc,
		PageSize:       pageSize,
		LocationFinder: lf,
		MaxResourceAge: maxResourceAge,
		secretKey:      secretKey,
	}
	tpl, err := newTpl(template.FuncMap{
		"min":        minFunc(s.MaxResourceAge),
		"max":        max,
		"has_prefix": strings.HasPrefix,
	}, base+alertListTpl+pagingTpl)
	if err != nil {
		return nil, err
	}
	s.tpl = tpl
	return s, nil
}

func (s *alertListServer) renderError(w http.ResponseWriter, r *http.Request, code int, query url.Values, err error) {
	if err == nil {
		panic("called renderError with a nil error")
	}
	str := strings.Replace(err.Error(), "twilio: ", "", 1)
	data := &baseData{
		LF: s.LocationFinder,
		Data: &alertListData{
			Err:   str,
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

func (s *alertListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewAlerts() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
		return
	}
	ctx, cancel := getContext(r.Context(), 3*time.Second)
	defer cancel()
	query := r.URL.Query()
	opaqueNext := query.Get("next")
	page := new(views.AlertPage)
	var err error
	start := time.Now()
	if opaqueNext != "" {
		next, nextErr := services.Unopaque(opaqueNext, s.secretKey)
		if nextErr != nil {
			err = errors.New("Could not decrypt `next` query parameter: " + nextErr.Error())
			s.renderError(w, r, http.StatusBadRequest, query, err)
			return
		}
		if !strings.HasPrefix(next, twilio.MonitorBaseURL) {
			s.Warn("Invalid next page URI", "next", next, "opaque", opaqueNext)
			s.renderError(w, r, http.StatusBadRequest, query, errors.New("Invalid next page uri"))
			return
		}
		page, err = s.Client.GetNextAlertPage(ctx, u, next)
		setNextPageValsOnQuery(next, query)
	} else {
		vals := url.Values{}
		vals.Set("PageSize", strconv.FormatUint(uint64(s.PageSize), 10))
		if filterErr := setPageFilters(query, vals); filterErr != nil {
			s.renderError(w, r, http.StatusBadRequest, query, filterErr)
			return
		}
		page, err = s.Client.GetAlertPage(ctx, u, vals)
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
	// Fetch the next page into the cache
	go func(u *config.User, n types.NullString) {
		if n.Valid {
			if _, err := s.Client.GetNextAlertPage(context.Background(), u, n.String); err != nil {
				s.Debug("Error fetching next page", "err", err)
			}
		}
	}(u, page.NextPageURI())
	data := &baseData{
		LF:       s.LocationFinder,
		Duration: time.Since(start),
	}
	data.Data = &alertListData{
		Page:                  page,
		Query:                 query,
		Loc:                   s.LocationFinder.GetLocationReq(r),
		EncryptedNextPage:     getEncryptedPage(page.NextPageURI(), s.secretKey),
		EncryptedPreviousPage: getEncryptedPage(page.PreviousPageURI(), s.secretKey),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	if err := render(w, r, s.tpl, "base", data); err != nil {
		rest.ServerError(w, r, err)
	}
}
