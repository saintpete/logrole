BUMP_VERSION := $(shell command -v bump_version)
GODOCDOC := $(shell command -v godocdoc)
GO_BINDATA := $(shell command -v go-bindata)

WATCH_TARGETS = static/css/style.css \
	cache/cache.go \
	commands/server/main.go \
	config/permission.go \
	templates/base.html templates/index.html templates/messages/list.html \
	templates/messages/instance.html templates/calls/list.html \
	templates/calls/instance.html templates/calls/recordings.html \
	templates/conferences/list.html \
	templates/errors.html templates/login.html \
	templates/snippets/phonenumber.html \
	services/error_reporter.go services/services.go \
	server/authenticator.go server/render.go server/tz.go \
	server/conferences.go \
	server/serve.go server/messages.go server/search.go server/images.go \
	server/calls.go server/page.go server/audio.go server/errors.go \
	views/message.go views/client.go views/call.go views/recording.go \
	views/conference.go \
	Makefile config.yml

ASSET_TARGETS = templates/base.html templates/messages/list.html \
	templates/messages/instance.html templates/calls/list.html \
	templates/calls/instance.html templates/calls/recordings.html \
	static/css/style.css static/css/bootstrap.min.css

test: vet
	go test -short ./...

race-test: vet
	go test -race ./...

serve:
	go run commands/server/main.go commands/server/config.go

vet:
	go vet ./assets/... ./cache/... ./commands/... ./config/... \
		./server/... ./services/... ./test/... ./views/...

deploy:
	git push heroku master

assets: $(ASSET_TARGETS)
ifndef GO_BINDATA
	go get -u github.com/jteeuwen/go-bindata/...
endif
	go-bindata -o=assets/bindata.go --pkg=assets templates/... static/...

watch:
	justrun --delay=100ms -c 'make assets serve' $(WATCH_TARGETS)

deps:
	godep save ./...

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
