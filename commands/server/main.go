package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/kevinburke/handlers"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/server"
	"github.com/saintpete/logrole/services"
	yaml "gopkg.in/yaml.v2"
)

const DefaultPort = "4114"
const DefaultPageSize = 50

// DefaultMaxResourceAge allows all resources to be fetched. The company was
// founded in 2008, so there should definitely be no resources created in the
// 1980's.
var DefaultMaxResourceAge = time.Since(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))

type config struct {
	Port             string        `yaml:"port"`
	AccountSid       string        `yaml:"twilio_account_sid"`
	AuthToken        string        `yaml:"twilio_auth_token"`
	User             string        `yaml:"basic_auth_user"`
	Password         string        `yaml:"basic_auth_password"`
	Realm            services.Rlm  `yaml:"realm"`
	Timezone         string        `yaml:"timezone"`
	PublicHost       string        `yaml:"public_host"`
	MessagesPageSize uint          `yaml:"messages_page_size"`
	SecretKey        string        `yaml:"secret_key"`
	MaxResourceAge   time.Duration `yaml:"max_resource_age"`
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

func main() {
	cfg := flag.String("config", "config.yml", "Path to a config file")
	flag.Parse()
	data, err := ioutil.ReadFile(*cfg)
	c := new(config)
	if err == nil {
		if err := yaml.Unmarshal(data, c); err != nil {
			handlers.Logger.Error("Couldn't parse config file", "err", err)
			os.Exit(2)
		}
	} else {
		if *cfg != "config.yml" {
			handlers.Logger.Error("Couldn't find config file", "err", err)
			os.Exit(2)
		}
		c.Port = DefaultPort
		c.Realm = services.Local
	}
	if c.SecretKey == "" {
		handlers.Logger.Warn("No secret key provided, generating random secret key. Sessions won't persist across restarts")
	}
	secretKey, err := getSecretKey(c.SecretKey)
	if err != nil {
		handlers.Logger.Error(err.Error(), "key", c.SecretKey)
		os.Exit(2)
	}
	if c.MaxResourceAge == 0 {
		c.MaxResourceAge = DefaultMaxResourceAge
	}
	if c.User == "" || c.Password == "" {
		handlers.Logger.Error("Cannot run without Basic Auth, set a basic_auth_user")
		os.Exit(2)
	}
	allowHTTP := false
	if c.Realm == services.Local {
		allowHTTP = true
	}
	client := twilio.NewClient(c.AccountSid, c.AuthToken, nil)
	users := make(map[string]string)
	if c.User != "" {
		users[c.User] = c.Password
	}
	var location *time.Location
	if c.Timezone == "" {
		handlers.Logger.Info("No timezone provided, defaulting to UTC")
		location = time.UTC
	} else {
		var err error
		location, err = time.LoadLocation(c.Timezone)
		if err != nil {
			handlers.Logger.Error("Couldn't find timezone", "err", err, "timezone", c.Timezone)
			os.Exit(2)
		}
	}
	if c.MessagesPageSize == 0 {
		c.MessagesPageSize = DefaultPageSize
	}

	settings := &server.Settings{
		AllowUnencryptedTraffic: allowHTTP,
		Users:            users,
		Client:           client,
		Location:         location,
		PublicHost:       c.PublicHost,
		MessagesPageSize: c.MessagesPageSize,
		SecretKey:        secretKey,
		MaxResourceAge:   c.MaxResourceAge,
	}
	s := server.NewServer(settings)
	publicMux := http.NewServeMux()
	publicMux.Handle("/", s)
	publicServer := http.Server{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		Handler:      publicMux,
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", c.Port))
	if err != nil {
		handlers.Logger.Error("Error listening", "err", err, "port", c.Port)
		os.Exit(2)
	}
	go func(p string) {
		time.Sleep(30 * time.Millisecond)
		handlers.Logger.Info("Started server", "port", p)
	}(c.Port)
	publicServer.Serve(listener)
}
