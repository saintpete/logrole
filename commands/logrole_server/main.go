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

	log "github.com/inconshreveable/log15"
	"github.com/kevinburke/handlers"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/server"
	"github.com/saintpete/logrole/services"
	yaml "gopkg.in/yaml.v2"
)

var logger log.Logger

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

	logger = handlers.Logger
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
	c := new(config.FileConfig)
	if err == nil {
		if err := yaml.Unmarshal(data, c); err != nil {
			logger.Error("Couldn't parse config file", "err", err)
			os.Exit(2)
		}
	} else {
		if *cfg != "config.yml" {
			logger.Error("Couldn't find config file", "err", err)
			os.Exit(2)
		}
		logger.Warn("Couldn't find config file, defaulting to localhost:4114")
		c.Port = config.DefaultPort
		c.Realm = services.Local
	}
	if c.Debug {
		logger = handlers.NewLoggerLevel(log.LvlDebug)
	}
	settings, err := config.NewSettingsFromConfig(c, logger)
	if err != nil {
		logger.Error("Error loading settings from config", "err", err)
		os.Exit(2)
	}
	s, err := server.NewServer(settings)
	if err != nil {
		logger.Error("Error creating the server", "err", err)
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
		logger.Error("Error listening", "err", err, "port", c.Port)
		os.Exit(2)
	}
	go func(p string) {
		time.Sleep(30 * time.Millisecond)
		logger.Info("Started server", "port", p, "public_host", settings.PublicHost)
	}(c.Port)
	publicServer.Serve(listener)
}
