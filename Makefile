BUMP_VERSION := $(shell command -v bump_version)
GODOCDOC := $(shell command -v godocdoc)
GO_BINDATA := $(shell command -v go-bindata)

WATCH_TARGETS = static/css/style.css \
	commands/server/main.go \
	config/permission.go \
	templates/base.html templates/messages/list.html \
	templates/messages/instance.html templates/calls/list.html \
	templates/calls/instance.html templates/calls/recordings.html \
	server/serve.go server/messages.go server/search.go server/images.go \
	server/calls.go server/page.go server/audio.go \
	views/message.go views/client.go views/call.go views/recording.go

ASSET_TARGETS = templates/base.html templates/messages/list.html \
	templates/messages/instance.html templates/calls/list.html \
	templates/calls/instance.html templates/calls/recordings.html \
	static/css/style.css static/css/bootstrap.min.css

test: vet
	go test -short ./...

race-test: vet
	go test -race ./...

serve:
	go run commands/server/main.go

vet:
	go vet ./assets/... ./commands/... ./config/... ./server/... ./services/... ./views/...

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
