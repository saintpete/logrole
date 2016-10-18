package server

import (
	"net/http"

	"github.com/saintpete/logrole/services"
)

func Example() {
	settings := &Settings{
		PublicHost: "myapp.com",
		SecretKey:  services.NewRandomKey(),
	}
	s := NewServer(settings)
	http.Handle("/", s)
	http.ListenAndServe(":4114", nil)
}
