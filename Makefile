serve:
	go run commands/server/*.go --user=test --password=test

test: vet
	go test -short ./server/... ./commands/...

vet:
	go vet ./server/... ./commands/...

deploy: 
	git push heroku master

assets: templates/sms.html static/css/bootstrap.min.css
	go-bindata -o=assets/bindata.go --pkg=assets templates/... static/...

watch:
	justrun -c 'make assets serve' templates/sms.html server/serve.go
