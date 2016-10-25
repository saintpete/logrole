package main

import (
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/kevinburke/handlers"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/server"
	"github.com/saintpete/logrole/services"
)

type fileConfig struct {
	Port           string        `yaml:"port"`
	AccountSid     string        `yaml:"twilio_account_sid"`
	AuthToken      string        `yaml:"twilio_auth_token"`
	Realm          services.Rlm  `yaml:"realm"`
	Timezone       string        `yaml:"timezone"`
	PublicHost     string        `yaml:"public_host"`
	PageSize       uint          `yaml:"page_size"`
	SecretKey      string        `yaml:"secret_key"`
	MaxResourceAge time.Duration `yaml:"max_resource_age"`

	// Need a pointer to a boolean here since we want to be able to distinguish
	// "false" from "omitted"
	ShowMediaByDefault *bool `yaml:"show_media_by_default,omitempty"`

	EmailAddress string `yaml:"email_address"`

	ErrorReporter      string `yaml:"error_reporter,omitempty"`
	ErrorReporterToken string `yaml:"error_reporter_token,omitempty"`

	AuthScheme string `yaml:"auth_scheme"`
	User       string `yaml:"basic_auth_user"`
	Password   string `yaml:"basic_auth_password"`

	GoogleClientID     string `yaml:"google_client_id"`
	GoogleClientSecret string `yaml:"google_client_secret"`
}

func NewSettingsFromConfig(c *fileConfig) (*server.Settings, error) {
	allowHTTP := false
	if c.Realm == services.Local {
		allowHTTP = true
	}
	if c.SecretKey == "" {
		handlers.Logger.Warn("No secret key provided, generating random secret key. Sessions won't persist across restarts")
	}
	secretKey, err := getSecretKey(c.SecretKey)
	if err != nil {
		return nil, err
	}
	if c.MaxResourceAge == 0 {
		c.MaxResourceAge = config.DefaultMaxResourceAge
	}
	var address *mail.Address
	if c.EmailAddress != "" {
		address, err = mail.ParseAddress(c.EmailAddress)
		if err != nil {
			return nil, fmt.Errorf("Couldn't parse email address: %v", err)
		}
	}
	if c.ErrorReporter != "" {
		if !services.IsRegistered(c.ErrorReporter) {
			handlers.Logger.Warn("Unknown error reporter, using the noop reporter", "name", c.ErrorReporter)
		}
	}
	reporter := services.GetReporter(c.ErrorReporter, c.ErrorReporterToken)
	var authenticator server.Authenticator
	switch c.AuthScheme {
	case "":
		handlers.Logger.Warn("Disabling basic authentication")
		authenticator = &server.NoopAuthenticator{}
	case "basic":
		if c.User == "" || c.Password == "" {
			return nil, errors.New("Cannot use basic auth without a username or password, set a basic_auth_user")
		}
		users := make(map[string]string)
		if c.User != "" {
			users[c.User] = c.Password
		}
		authenticator = server.NewBasicAuthAuthenticator("logrole", users)
	case "google":
		var baseURL string
		if allowHTTP {
			baseURL = "http://" + c.PublicHost
		} else {
			baseURL = "https://" + c.PublicHost
		}
		gauthenticator := server.NewGoogleAuthenticator(c.GoogleClientID, c.GoogleClientSecret, baseURL, secretKey)
		gauthenticator.AllowUnencryptedTraffic = allowHTTP
		authenticator = gauthenticator
	default:
		return nil, fmt.Errorf("Unknown auth scheme: %s", c.AuthScheme)
	}
	client := twilio.NewClient(c.AccountSid, c.AuthToken, nil)
	if c.Timezone == "" {
		handlers.Logger.Info("No timezone provided, defaulting to UTC")
	}
	locationFinder, err := services.NewLocationFinder(c.Timezone)
	if err != nil {
		return nil, fmt.Errorf("Couldn't find timezone %s: %s", c.Timezone, err.Error())
	}
	for _, timezone := range timezones {
		if ok := locationFinder.AddLocation(timezone); !ok {
			handlers.Logger.Warn("Couldn't add location", "tz", timezone)
		}
	}
	// TODO
	if c.PageSize == 0 {
		c.PageSize = config.DefaultPageSize
	}
	if c.PageSize > 1000 {
		return nil, fmt.Errorf("Maximum allowable page size is 1000, got %d", c.PageSize)
	}
	if c.ShowMediaByDefault == nil {
		b := true
		c.ShowMediaByDefault = &b
	}

	settings := &server.Settings{
		AllowUnencryptedTraffic: allowHTTP,
		Client:                  client,
		LocationFinder:          locationFinder,
		PublicHost:              c.PublicHost,
		PageSize:                c.PageSize,
		SecretKey:               secretKey,
		MaxResourceAge:          c.MaxResourceAge,
		ShowMediaByDefault:      *c.ShowMediaByDefault,
		Mailto:                  address,
		Reporter:                reporter,
		Authenticator:           authenticator,
	}
	return settings, nil
}
