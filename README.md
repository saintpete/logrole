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

## Local Development

Logrole is written in Go; you'll need a working Go environment
running at least Go 1.7. Follow the instructions here to set one up:
https://golang.org/doc/install

To check out the project, run `go get github.com/saintpete/logrole/...`.

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
