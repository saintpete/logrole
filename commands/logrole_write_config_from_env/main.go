package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/saintpete/logrole/server"
)

func init() {
	flag.Usage = func() {
		os.Stderr.WriteString(`logrole_write_config_from_env

Read configuration from environment variables and write it to a yml file. By
default this script prints the config to stdout. Pass --config=<file> to write
to a file instead.

Supported environment variables are:

PORT                   Port to listen on
PUBLIC_HOST            Host your users will browse to to see the site

TWILIO_ACCOUNT_SID     Account SID for your Twilio account
TWILIO_AUTH_TOKEN      Auth token

REALM                  Realm (either "local" or "prod")
TZ                     Default timezone (example "America/Los_Angeles")
TIMEZONES              Comma-separated list of timezones users can choose from
                       (example "America/New_York,UTC,America/Chicago"). 
                       Defaults to the four US timezones.
EMAIL_ADDRESS          For "Contact Support" on server error pages
PAGE_SIZE              How many resources to fetch/display on each page

SECRET_KEY             64 byte hex key - generate with "openssl rand -hex 32"
MAX_RESOURCE_AGE       How long resources should be visible for - "720h" to
                       hide anything older than 30 days
SHOW_MEDIA_BY_DEFAULT  "false" to hide images behind a toggle when a user
                       browses to a MMS message.

AUTH_SCHEME            "basic", "noop", or "google"
BASIC_AUTH_USER        For basic auth, the username
BASIC_AUTH_PASSWORD    For basic auth, the password
GOOGLE_CLIENT_ID       For Google OAuth
GOOGLE_CLIENT_SECRET   For Google OAuth
GOOGLE_ALLOWED_DOMAINS Comma separated list of domains to allow to
                       authenticate. If empty or omitted, all domains allowed.

ERROR_REPORTER         "sentry", empty, or register your own.
ERROR_REPORTER_TOKEN   Token for the error reporter.

Usage of write_config_from_env:
`)
		flag.PrintDefaults()
		os.Exit(2)
	}
}

// environment facilitates finding environment variables - an interface so we
// can mock it out in tests.
type environment interface {
	LookupEnv(string) (string, bool)
}

type osEnvironment struct{}

func (o *osEnvironment) LookupEnv(val string) (string, bool) {
	return os.LookupEnv(val)
}

// writeVal writes a YAML configuration for the given environment variable.
// Returns true if the value was successfully written, and quits if Write()
// fails with an error.
func writeVal(w io.Writer, e environment, env string, cfgval string) bool {
	if v, ok := e.LookupEnv(env); ok {
		_, err := fmt.Fprintln(w, cfgval+":", v)
		checkErr(err, "writing config")
		return true
	}
	return false
}

func checkErr(err error, activity string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error %s: %s\n", activity, err.Error())
		os.Exit(2)
	}
}

func writeCommaSeparatedVal(w io.Writer, e environment, env string, cfgval string) bool {
	if v, ok := e.LookupEnv(env); ok {
		_, err := fmt.Fprintf(w, "%s:\n", cfgval)
		checkErr(err, "writing config")
		vals := strings.Split(v, ",")
		for _, val := range vals {
			_, err := fmt.Fprintf(w, "  - %s\n", strings.TrimSpace(val))
			checkErr(err, "writing config")
		}
		return true
	}
	return false
}

func writeConfig(b *bytes.Buffer, e environment) {
	var ok bool
	ok = writeVal(b, e, "PORT", "port") || ok
	ok = writeVal(b, e, "PUBLIC_HOST", "public_host") || ok
	if ok {
		b.WriteByte('\n')
		ok = false
	}
	ok = writeVal(b, e, "TWILIO_ACCOUNT_SID", "twilio_account_sid") || ok
	ok = writeVal(b, e, "TWILIO_AUTH_TOKEN", "twilio_auth_token") || ok
	if ok {
		b.WriteByte('\n')
		ok = false
	}
	ok = writeVal(b, e, "REALM", "realm") || ok
	ok = writeVal(b, e, "TZ", "default_timezone") || ok
	ok = writeCommaSeparatedVal(b, e, "TIMEZONES", "timezones") || ok
	ok = writeVal(b, e, "EMAIL_ADDRESS", "email_address") || ok
	ok = writeVal(b, e, "PAGE_SIZE", "page_size") || ok
	if ok {
		b.WriteByte('\n')
		ok = false
	}
	ok = writeVal(b, e, "SECRET_KEY", "secret_key") || ok
	ok = writeVal(b, e, "MAX_RESOURCE_AGE", "max_resource_age") || ok
	ok = writeVal(b, e, "SHOW_MEDIA_BY_DEFAULT", "show_media_by_default") || ok
	if ok {
		b.WriteByte('\n')
		ok = false
	}
	ok = writeVal(b, e, "AUTH_SCHEME", "auth_scheme") || ok
	ok = writeVal(b, e, "BASIC_AUTH_USER", "basic_auth_user") || ok
	ok = writeVal(b, e, "BASIC_AUTH_PASSWORD", "basic_auth_password") || ok
	ok = writeVal(b, e, "GOOGLE_CLIENT_ID", "google_client_id") || ok
	ok = writeVal(b, e, "GOOGLE_CLIENT_SECRET", "google_client_secret") || ok
	ok = writeCommaSeparatedVal(b, e, "GOOGLE_ALLOWED_DOMAINS", "google_allowed_domains") || ok
	if ok {
		b.WriteByte('\n')
		ok = false
	}
	ok = writeVal(b, e, "ERROR_REPORTER", "error_reporter") || ok
	ok = writeVal(b, e, "ERROR_REPORTER_TOKEN", "error_reporter_token") || ok
}

func main() {
	cfg := flag.String("config", "", "Path to a config file")
	flag.Parse()
	if flag.NArg() == 1 {
		switch flag.Arg(0) {
		case "version":
			fmt.Fprintf(os.Stderr, "logrole version %s\n", server.Version)
			os.Exit(2)
		case "help":
			flag.Usage()
		default:
			fmt.Fprintf(os.Stderr, "Unknown argument: %s\n", flag.Arg(0))
			os.Exit(2)
		}
	}
	b := new(bytes.Buffer)
	writeConfig(b, &osEnvironment{})
	var w io.Writer
	if *cfg == "" {
		w = os.Stdout
	} else {
		f, err := os.Create(*cfg)
		checkErr(err, "creating config file")
		defer f.Close()
		w = f
	}
	_, err := io.Copy(w, b)
	checkErr(err, "writing config file")
}
