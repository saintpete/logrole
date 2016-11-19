# would be great to make the bash location portable but not sure how
SHELL = /bin/bash

BUMP_VERSION := $(shell command -v bump_version)
GODOCDOC := $(shell command -v godocdoc)
GO_BINDATA := $(shell command -v go-bindata)
GOVENDOR := $(shell command -v govendor)
JUSTRUN := $(shell command -v justrun)
BENCHSTAT := $(shell command -v benchstat)
WRITE_MAILMAP := $(shell command -v write_mailmap)
STATICCHECK := $(shell command -v staticcheck)

WATCH_TARGETS = static/css/style.css \
	templates/phone-numbers/list.html templates/phone-numbers/instance.html \
	templates/conferences/instance.html templates/conferences/list.html \
	templates/alerts/list.html templates/alerts/instance.html \
	templates/errors.html templates/login.html \
	templates/snippets/phonenumber.html \
	services/error_reporter.go services/services.go \
	server/calls.go server/alerts.go server/phonenumbers.go \
	server/serve.go server/render.go views/client.go views/numbers.go \
	Makefile config.yml

ASSET_TARGETS = templates/base.html templates/index.html \
	templates/messages/list.html templates/messages/instance.html \
	templates/calls/list.html templates/calls/instance.html \
	templates/calls/recordings.html \
	templates/conferences/list.html templates/conferences/instance.html \
	templates/alerts/list.html templates/alerts/instance.html \
	templates/phone-numbers/list.html \
	templates/snippets/phonenumber.html \
	templates/errors.html templates/login.html \
	static/css/style.css static/css/bootstrap.min.css

test: vet
	@# this target should always be listed first so "make" runs the tests.
	go list ./... | grep -v vendor | xargs go test -short

race-test: vet
	go list ./... | grep -v vendor | xargs go test -race

serve:
	go run commands/logrole_server/main.go

vet:
ifndef STATICCHECK
	go get -u honnef.co/go/staticcheck/cmd/staticcheck
endif
	@# We can't vet the vendor directory, it fails.
	go list ./... | grep -v vendor | xargs go vet
	go list ./... | grep -v vendor | xargs staticcheck

deploy:
	git push heroku master

compile-css: static/css/bootstrap.min.css static/css/style.css
	cat static/css/bootstrap.min.css static/css/style.css > static/css/all.css

assets: $(ASSET_TARGETS) compile-css
ifndef GO_BINDATA
	go get -u github.com/jteeuwen/go-bindata/...
endif
	go-bindata -o=assets/bindata.go --nometadata --pkg=assets templates/... static/...

watch:
ifndef JUSTRUN
	go get -u github.com/jmhodges/justrun
endif
	justrun -v --delay=100ms -c 'make assets serve' $(WATCH_TARGETS)

deps:
ifndef GOVENDOR
	go get -u github.com/kardianos/govendor
endif
	govendor sync

release: race-test
ifndef BUMP_VERSION
	go get github.com/Shyp/bump_version
endif
	bump_version minor server/serve.go

docs:
ifndef GODOCDOC
	go get github.com/kevinburke/godocdoc
endif
	godocdoc

bench:
ifndef BENCHSTAT
	go get rsc.io/benchstat
endif
	tmp=$$(mktemp); go list ./... | grep -v vendor | xargs go test -benchtime=2s -bench=. -run='^$$' > "$$tmp" 2>&1 && benchstat "$$tmp"

loc:
	cloc --exclude-dir=.git,tmp,vendor --not-match-f='bootstrap.min.css|all.css|bindata.go' .

authors:
ifndef WRITE_MAILMAP
	go get github.com/kevinburke/write_mailmap
endif
	write_mailmap > AUTHORS.txt
