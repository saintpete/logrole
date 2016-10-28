# logrole

Logrole is a faster, usable, fine-grained client for exploring your Twilio
logs.

- Customizable permissions for each user browsing the site - limit access to
SMS/MMS bodies, resources older than a certain age, recordings, calls, call
from, etc. etc.

- Your Account Sid is obscured from end users at all times.

- Easy site search - tab complete and search for a sid to go straight to the
  instance view for that resource.

- Click-to-copy sids and phone numbers.

- MMS messages are always fetched over HTTPS. The default Twilio API/libraries
hand back insecure image links, but we rewrite URLs before fetching them.

## Latency

Logrole fetches and caches the first page of every result set every 30 seconds,
and any time you page through records, the next page is prefetched and stored
in a cache. This means viewing your Twilio logs via Logrole is *significantly
faster* than viewing results in your Dashboard or via the API! If you don't
believe me, the request latencies are displayed on every page.

If you need to search your Twilio Logs, this is the way you should do it.

## Installation

To install Logrole, run

```bash
go get -u github.com/saintpete/logrole/...
```

You will need a working [Go environment][go-env]; I recommend setting the
GOPATH environment variable to `$HOME/go` in your .bashrc or equivalent.

```bash
export GOPATH="$HOME/go"
```

## Deployment

There are several ways to deploy Logrole.

### As a binary

Logrole comes with a `logrole_server` binary (in commands/logrole_server) that
can load configuration and start the server for you. The `logrole_server`
binary parses a YAML file containing all of the server's configuration (an
example configuration file can be found in `config.sample.yml`). By default,
`logrole_server` looks for a file named `config.yml` in the same directory as
its cwd. Alternatively, pass the `--config` flag to the binary.

```bash
logrole_server --config=/path/to/myconfig.yml
```

### Via environment variables

Some environments like Heroku only allow you to set production
configuration via environment variables. Logrole has a second binary,
`logrole_write_config_from_env`, that takes named environment variables and
writes them to a YAML file. You can then run the `logrole_server` binary as
described above.

```
logrole_write_config_from_env > myconfig.yml
logrole_server --config=myconfig.yml
```

To see which environment variables are written to the file, run
`logrole_write_config_from_env --help`. Here is an example:

```
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
```

Logrole comes with a default Procfile and a start script (`bin/serve`) for easy
Heroku deployment. Sensitive environment variables (auth token, basic auth
password, etc) are dropped before the server process starts.

### As a Go library

Follow these instructions if you want to load Logrole from other Go
code, or create your own custom binary for running Logrole. Create [a
`server.Settings` struct][settings-godoc]. You may want to look at
`commands/logrole_server/config.go` for an example of how to build the Settings
struct with reasonable defaults.

```go
settings := &server.Settings{PublicHost: "example.com", PageSize: 50}
```

Once you have a Settings object, get a Server, then you can do what you want to
listen on ports or run tests or anything.

```go
s := server.NewServer(settings)
http.Handle("/", s)
http.ListenAndServe(":4114", nil)
```

[settings-godoc]: https://godoc.org/github.com/saintpete/logrole/server/#Settings

## Local Development

Logrole is written in Go; you'll need a [working Go environment][go-env]
running at least Go 1.7. Follow the instructions here to set one up:
https://golang.org/doc/install.

[go-env]: https://golang.org/doc/install

To check out the project, run `go get -u github.com/saintpete/logrole/...`.

To start a development server, run `make serve`. This will start a server on
[localhost:4114](http://localhost:4114).

By default we look for a config file in `config.yml`. The values for this
config file can be found in `commands/logrole_server/main.go`. There's an
example config file at config.sample.yml.

## Run the tests

Please run the tests before submitting any changes. Run `make test` to run the
tests, or run `make race-test` to run the tests with the race detector enabled.

## View the documentation

Run `make docs`.

## Errata

The Twilio Dashboard displays Participants for completed Conferences, but [this
functionality is not available via the API][issue-4]. Please [contact Support
to request this feature][support].

[support]: mailto:help@twilio.com
[issue-4]: https://github.com/saintpete/logrole/issues/4

The Start/End date filters may only work in Chrome.
