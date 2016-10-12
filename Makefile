serve:
	go run commands/server/*.go

test: vet
	go test -short ./server/... ./commands/...

vet:
	go vet ./server/... ./commands/...

deploy: 
	git push heroku master

assets: templates/messages.html static/css/style.css static/css/bootstrap.min.css
	go-bindata -o=assets/bindata.go --pkg=assets templates/... static/...

watch:
	justrun -c 'make assets serve' static/css/style.css commands/server/main.go templates/messages.html server/serve.go

deps:
	godep save ./...
