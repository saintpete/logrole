vet:
	go vet ./...

test: vet
	go test ./...

install:
	go install ./...

release: test
	bump_version minor lib.go
