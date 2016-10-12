package main

import (
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

type config struct {
	Port       string       `yaml:"port"`
	AccountSid string       `yaml:"twilio_account_sid"`
	AuthToken  string       `yaml:"twilio_auth_token"`
	User       string       `yaml:"basic_auth_user"`
	Password   string       `yaml:"basic_auth_password"`
	Realm      services.Rlm `yaml:"realm"`
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
	if c.Realm == services.Prod && (c.User == "" || c.Password == "") {
		handlers.Logger.Error("Cannot run in production without Basic Auth")
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
	s := server.NewServer(allowHTTP, users, client)
	publicMux := http.NewServeMux()
	publicMux.Handle("/", s)
	publicServer := http.Server{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		Handler: handlers.Log(
			handlers.Debug(
				handlers.UUID(
					handlers.Server(publicMux, "logrole"),
				),
			),
		),
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
