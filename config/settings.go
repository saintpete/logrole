package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/mail"
	"time"

	"github.com/kevinburke/handlers"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/services"
	yaml "gopkg.in/yaml.v2"
)

const DefaultPort = "4114"
const DefaultPageSize = 50

var DefaultTimezones = []string{
	"America/Los_Angeles",
	"America/Denver",
	"America/Chicago",
	"America/New_York",
}

// DefaultMaxResourceAge allows all resources to be fetched. The company was
// founded in 2008, so there should definitely be no resources created in the
// 1980's.
var DefaultMaxResourceAge = time.Since(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))

// FileConfig defines the settings you can load from a YAML configuration file.
// Load configuration from a YAML file into a FileConfig struct, then call
// NewSettingsFromConfig to get a Settings object.
//
// All of the types and values here should be representable in a YAML file.
type FileConfig struct {
	Port       string       `yaml:"port"`
	AccountSid string       `yaml:"twilio_account_sid"`
	AuthToken  string       `yaml:"twilio_auth_token"`
	Realm      services.Rlm `yaml:"realm"`
	// Default timezone for dates/times in the UI
	Timezone string `yaml:"default_timezone"`
	// List of timezones a user can choose in the UI
	Timezones      []string      `yaml:"timezones"`
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

	GoogleClientID       string   `yaml:"google_client_id"`
	GoogleClientSecret   string   `yaml:"google_client_secret"`
	GoogleAllowedDomains []string `yaml:"google_allowed_domains"`

	PolicyFile string `yaml:"policy_file"`
	Policy     *Policy

	Debug bool `yaml:"debug"`
}

// Settings are used to configure a Server and apply to all of the website's
// users.
type Settings struct {
	// The host the user visits to get to this site.
	PublicHost              string
	AllowUnencryptedTraffic bool
	Client                  *twilio.Client

	LocationFinder services.LocationFinder

	// How many messages to display per page.
	PageSize uint

	// Used to encrypt next page URI's and sessions. See config.sample.yml for
	// more information.
	SecretKey *[32]byte

	// Don't show resources that are older than this age.
	MaxResourceAge time.Duration

	// Should a user have to click a button to view media attached to a MMS?
	ShowMediaByDefault bool

	// Email address for server errors / "contact me" on error pages.
	Mailto *mail.Address

	// Error reporter. This must not be nil; set to NoopErrorReporter to ignore
	// errors.
	Reporter services.ErrorReporter

	// The authentication scheme.
	Authenticator Authenticator
}

var errWrongLength = errors.New("Secret key has wrong length. Should be a 64-byte hex string")

// getSecretKey produces a valid [32]byte secret key or returns an error. If
// hexKey is the empty string, a valid 32 byte key will be randomly generated
// and returned. If hexKey is invalid, an error is returned.
func getSecretKey(hexKey string) (*[32]byte, error) {
	if hexKey == "" {
		return services.NewRandomKey(), nil
	}

	if len(hexKey) != 64 {
		return nil, errWrongLength
	}
	secretKeyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	secretKey := new([32]byte)
	copy(secretKey[:], secretKeyBytes)
	return secretKey, nil
}

func NewSettingsFromConfig(c *FileConfig) (settings *Settings, err error) {
	defer func() {
		if r := recover(); r != nil {
			if c.Debug {
				panic(r)
			}
			err = fmt.Errorf("Panic in NewSettings: %#v", r)
			settings = nil
			return
		}
	}()
	logger := handlers.Logger
	if c.Policy != nil && c.PolicyFile != "" {
		return nil, errors.New("Cannot define both policy and a policy_file")
	}
	allowHTTP := false
	if c.Realm == services.Local {
		allowHTTP = true
	}
	if c.SecretKey == "" {
		logger.Warn("No secret key provided, generating random secret key. Sessions won't persist across restarts")
	}
	secretKey, err := getSecretKey(c.SecretKey)
	if err != nil {
		return nil, err
	}
	if c.MaxResourceAge == 0 {
		c.MaxResourceAge = DefaultMaxResourceAge
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
			logger.Warn("Unknown error reporter, using the noop reporter", "name", c.ErrorReporter)
		}
	}
	reporter := services.GetReporter(c.ErrorReporter, c.ErrorReporterToken)

	if c.PolicyFile != "" {
		// we checked above that Policy is nil in this case
		data, err := ioutil.ReadFile(c.PolicyFile)
		if err != nil {
			logger.Error("Couldn't load permission file", "loc", c.PolicyFile)
			return nil, err
		}
		var policy Policy
		if err := yaml.Unmarshal(data, &policy); err != nil {
			logger.Error("Couldn't parse policy file", "err", err, "loc", c.PolicyFile)
			return nil, err
		}
		c.Policy = &policy
	}

	if c.Policy != nil {
		if err := validatePolicy(c.Policy); err != nil {
			logger.Error("Couldn't validate policy", "err", err)
			return nil, err
		}
	}
	var authenticator Authenticator
	switch c.AuthScheme {
	case "":
		logger.Warn("Disabling basic authentication")
		authenticator = &NoopAuthenticator{User: DefaultUser}
	case "basic":
		if c.User == "" || c.Password == "" {
			return nil, errors.New("Cannot use basic auth without a username or password, set a basic_auth_user")
		}
		ba := NewBasicAuthAuthenticator("logrole")
		ba.AddUserPassword(c.User, c.Password)
		authenticator = ba
	case "google":
		var baseURL string
		if allowHTTP {
			baseURL = "http://" + c.PublicHost
		} else {
			baseURL = "https://" + c.PublicHost
		}
		gauthenticator := NewGoogleAuthenticator(c.GoogleClientID, c.GoogleClientSecret, baseURL, c.GoogleAllowedDomains, secretKey)
		gauthenticator.AllowUnencryptedTraffic = allowHTTP
		authenticator = gauthenticator
	default:
		return nil, fmt.Errorf("Unknown auth scheme: %s", c.AuthScheme)
	}
	authenticator.SetPolicy(c.Policy)
	client := twilio.NewClient(c.AccountSid, c.AuthToken, nil)
	if c.Timezone == "" {
		logger.Info("No timezone provided, defaulting to UTC")
	}
	locationFinder, err := services.NewLocationFinder(c.Timezone)
	if err != nil {
		return nil, fmt.Errorf("Couldn't find timezone %s: %s", c.Timezone, err.Error())
	}
	tzs := DefaultTimezones
	if len(c.Timezones) > 0 {
		tzs = c.Timezones
	}
	for _, timezone := range tzs {
		if ok := locationFinder.AddLocation(timezone); !ok {
			logger.Warn("Couldn't add location", "tz", timezone)
		}
	}
	// TODO
	if c.PageSize == 0 {
		c.PageSize = DefaultPageSize
	}
	if c.PageSize > 1000 {
		return nil, fmt.Errorf("Maximum allowable page size is 1000, got %d", c.PageSize)
	}
	if c.ShowMediaByDefault == nil {
		b := true
		c.ShowMediaByDefault = &b
	}

	settings = &Settings{
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
	return
}
