// Command line binary for loading configuration and starting/running the
// logrole server.
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
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/server"
	"github.com/saintpete/logrole/services"
	yaml "gopkg.in/yaml.v2"
)

// TODO
var timezones = []string{
	"America/Los_Angeles",
	"America/Denver",
	"America/Chicago",
	"America/New_York",
}

func init() {
	flag.Usage = func() {
		os.Stderr.WriteString(`Logrole: a faster, finer-grained Twilio log viewer

Configuration should be written to a file (default config.yml in the 
current directory) and passed to the binary via the --config flag.

Usage of server:
`)
		flag.PrintDefaults()
		os.Exit(2)
	}
}

func main() {
	cfg := flag.String("config", "config.yml", "Path to a config file")
	flag.Parse()
	if flag.NArg() > 2 {
		os.Stderr.WriteString("too many arguments")
		os.Exit(2)
	}
	if flag.NArg() == 1 {
		switch flag.Arg(0) {
		case "version":
			fmt.Fprintf(os.Stderr, "logrole version %s (twilio-go version %s)\n", server.Version, twilio.Version)
			os.Exit(2)
		case "help":
			flag.Usage()
		case "serve":
			break
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", flag.Arg(0))
			os.Exit(2)
		}
	}
	data, err := ioutil.ReadFile(*cfg)
	c := new(fileConfig)
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
		handlers.Logger.Warn("Couldn't find config file, defaulting to localhost:4114")
		c.Port = config.DefaultPort
		c.Realm = services.Local
	}
	settings, err := NewSettingsFromConfig(c)
	if err != nil {
		handlers.Logger.Error("Error loading settings from config", "err", err)
		os.Exit(2)
	}
	s, err := server.NewServer(settings)
	if err != nil {
		handlers.Logger.Error("Error creating the server", "err", err)
		os.Exit(2)
	}
	s.CacheCommonQueries()
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
		handlers.Logger.Info("Started server", "port", p, "public_host", settings.PublicHost)
	}(c.Port)
	publicServer.Serve(listener)
}
