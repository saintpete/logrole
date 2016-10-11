package server

import "net/http"

type server struct{}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
	_, err := w.Write([]byte("Hello World"))
	if err != nil {
		panic(err)
	}
}

var Server = &server{}
