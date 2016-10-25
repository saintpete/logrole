package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/saintpete/logrole/server"
)

func init() {
	flag.Usage = func() {
		os.Stderr.WriteString(`write_config_from_env

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
EMAIL_ADDRESS          For "Contact Support" on server error pages
PAGE_SIZE              How many resources to fetch from Twilio/display on each page

SECRET_KEY             64 byte hex key - generate with "openssl rand -hex 32"
MAX_RESOURCE_AGE       How long resources should be visible for - "720h" to hide 
                       anything older than 30 days
SHOW_MEDIA_BY_DEFAULT  "false" to hide images behind a toggle when a user 
                       browses to a MMS message.

AUTH_SCHEME            "basic", "noop", or "google"
BASIC_AUTH_USER        For basic auth, the username
BASIC_AUTH_PASSWORD    For basic auth, the password
GOOGLE_CLIENT_ID       For Google OAuth
GOOGLE_CLIENT_SECRET   For Google OAuth

ERROR_REPORTER         "sentry", empty, or register your own.
ERROR_REPORTER_TOKEN   Token for the error reporter.

Usage of write_config_from_env:
`)
		flag.PrintDefaults()
		os.Exit(2)
	}
}

func writeVal(w io.Writer, env string, cfgval string) {
	if v, ok := os.LookupEnv(env); ok {
		fmt.Fprintln(w, cfgval+":", v)
	}
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
	writeVal(b, "PORT", "port")
	writeVal(b, "PUBLIC_HOST", "public_host")
	b.WriteByte('\n')
	writeVal(b, "TWILIO_ACCOUNT_SID", "twilio_account_sid")
	writeVal(b, "TWILIO_AUTH_TOKEN", "twilio_auth_token")
	b.WriteByte('\n')
	writeVal(b, "REALM", "realm")
	writeVal(b, "TZ", "timezone")
	writeVal(b, "EMAIL_ADDRESS", "email_address")
	writeVal(b, "PAGE_SIZE", "page_size")
	b.WriteByte('\n')
	writeVal(b, "SECRET_KEY", "secret_key")
	writeVal(b, "MAX_RESOURCE_AGE", "max_resource_age")
	writeVal(b, "SHOW_MEDIA_BY_DEFAULT", "show_media_by_default")
	b.WriteByte('\n')
	writeVal(b, "AUTH_SCHEME", "auth_scheme")
	writeVal(b, "BASIC_AUTH_USER", "basic_auth_user")
	writeVal(b, "BASIC_AUTH_PASSWORD", "basic_auth_password")
	writeVal(b, "GOOGLE_CLIENT_ID", "google_client_id")
	writeVal(b, "GOOGLE_CLIENT_SECRET", "google_client_id")
	b.WriteByte('\n')
	writeVal(b, "ERROR_REPORTER", "error_reporter")
	writeVal(b, "ERROR_REPORTER_TOKEN", "error_reporter_token")
	var w io.Writer
	if *cfg == "" {
		w = os.Stdout
	} else {
		f, err := os.Create(*cfg)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(2)
		}
		defer f.Close()
		w = f
	}
	_, err := io.Copy(w, b)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
}
