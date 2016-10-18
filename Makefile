BUMP_VERSION := $(shell command -v bump_version)
GODOCDOC := $(shell command -v godocdoc)

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

assets: templates/base.html templates/messages/list.html templates/messages/instance.html static/css/style.css static/css/bootstrap.min.css
	go-bindata -o=assets/bindata.go --pkg=assets templates/... static/...

watch:
	justrun -c 'make assets serve' static/css/style.css commands/server/main.go config/permission.go templates/base.html templates/messages/list.html templates/messages/instance.html server/serve.go server/messages.go server/search.go server/images.go views/message.go

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
