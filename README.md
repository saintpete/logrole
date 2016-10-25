# logrole

Logrole is a faster, usable, fine-grained client for exploring your Twilio
logs. It's pretty fast - there are hardly any dependencies and the main
bottleneck for every request is making an API request.

- Customizable permissions for each user browsing the site - limit access to
SMS/MMS bodies, resources older than a certain age, recordings, calls, call
from, etc. etc.

- Your Account Sid is obscured from end users at all times.

- Easy site search - tab complete and search for a sid to go straight to the
  instance view for that resource.

- MMS messages are always fetched over HTTPS. The default Twilio API/libraries
hand back insecure image links, but we rewrite URLs before fetching them.

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

Logrole comes with a `server` binary (in commands/server) that can load
configuration and start the server for you. The `server` binary parses a YAML
file containing all of the server's configuration (an example configuration
file can be found in `config.sample.yml`). By default, `server` looks for a
file named `config.yml` in the same directory as it's cwd. Alternatively, pass
the `--config` flag to the binary.

```bash
server --config=/path/to/myconfig.yml
```

### Via environment variables

Some environments like Heroku only allow you to set production
configuration via environment variables. Logrole has a second binary,
`write_config_from_env`, that takes named environment variables and writes them
to a YAML file. You can then run the `server` binary as described above.

To see which environment variables are loaded, run `write_config_from_env
--help`.

Logrole comes with a default Procfile and a start script (`bin/serve`) for easy
Heroku deployment. Sensitive environment variables (auth token, basic auth
password, etc) are dropped before the server process starts.

### As a Go library

Follow these instructions if you want to load Logrole from other Go code, or
create your own custom binary for running Logrole. Create [a `server.Settings`
struct][settings-godoc]. You may want to look at `commands/server/config.go`
for an example of how to build the Settings struct.

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
config file can be found in commands/server/main.go. There's an example config
file at config.sample.yml.

## Run the tests

Please run the tests before submitting any changes. Run `make test` to run the
tests, or run `make race-test` to run the tests with the race detector enabled.

## View the documentation

Run `make docs`.

## Errata

The Start/End date filters may only work in Chrome.
