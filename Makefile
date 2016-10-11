serve:
	go run commands/server/*.go --user=test --password=test

test: vet
	go test -short ./server/... ./commands/...

vet:
	go vet ./server/... ./commands/...

deploy: 
	git push heroku master
