vet: 
	go vet ./...

test: vet
	go test ./...

race-test: vet
	go test -race ./...

release: race-test
	bump_version minor client.go
