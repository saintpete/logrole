serve:
	go run commands/server/*.go

test: vet
	go test -short ./server/... ./commands/...

vet:
	go vet ./server/... ./commands/...

deploy: 
	git push heroku master

assets: templates/base.html templates/messages/list.html templates/messages/instance.html static/css/style.css static/css/bootstrap.min.css
	go-bindata -o=assets/bindata.go --pkg=assets templates/... static/...

watch:
	justrun -c 'make assets serve' static/css/style.css commands/server/main.go templates/base.html templates/messages/list.html templates/messages/instance.html server/serve.go server/messages.go server/search.go

deps:
	godep save ./...

release: test
	go get github.com/Shyp/bump_version
	bump_version minor server/serve.go

docs:
	go get github.com/kevinburke/godocdoc
	godocdoc
