package server

import (
	"errors"
	"net/http"

	"github.com/kevinburke/rest"
	"github.com/saintpete/logrole/config"
)

type conferenceListServer struct {
	PageSize uint
}

func newConferenceListServer(pageSize uint) *conferenceListServer {
	return &conferenceListServer{
		PageSize: pageSize,
	}
}

func (c *conferenceListServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := config.GetUser(r)
	if !ok {
		rest.ServerError(w, r, errors.New("No user available"))
		return
	}
	if !u.CanViewCalls() {
		rest.Forbidden(w, r, &rest.Error{Title: "Access denied"})
		return
	}
}
