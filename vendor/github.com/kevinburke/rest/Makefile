vet: 
	go vet ./...

test: vet
	go test ./...

race-test: vet
	go test -race ./...

release: test
	bump_version minor client.go
