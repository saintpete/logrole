# logrole

LogRole for Twilio - Create User Roles for Limited Access to Twilio Logs

## Local Development

You'll need a working Go environment. Follow the instructions here to set one
up: https://golang.org/doc/install

To check out the project, run `go get github.com/saintpete/logrole/...`.

To start a development server, run `make serve`. This will start a server on
[localhost:4114](http://localhost:4114).

By default we look for a config file in `config.yml`. The values for this
config file can be found in commands/server/main.go. There's an example config
file at config.sample.yml.

## Errata

The Start/End date filters may only work in Chrome.
