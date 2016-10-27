I'm happy to look at any pull requests. Please double check that the tests pass
before submitting changes; run `make race-test` to run the tests.

The project has gotten really far without using Javascript, and I would like to
keep it that way.

## Working with Godep

The dependencies for this project move frequently. To add a new dependency,
check out the dependency locally using `go get <dependency-name>`. Then start
using it in the project - just add the dependency where you would use it - and
run `make deps`. This should add the dependency to the `vendor` directory.

To update a dependency, say, `github.com/kevinburke/twilio-go` - update the
project on your local fork, `$GOPATH/src/github.com/kevinburke/twilio-go` to
the new version you want. Then in the logrole project, run

```
godep update github.com/kevinburke/twilio-go
```

This should update the version of the project in the `vendor` directory, and
update Godeps.json to the latest version.
