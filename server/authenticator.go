package server

import (
	"net/http"

	log "github.com/inconshreveable/log15"
	"github.com/saintpete/logrole/config"
)

type logoutServer struct {
	log.Logger
	Authenticator config.Authenticator
}

func (l *logoutServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.Authenticator.Logout(w, r)
}

// TODO add different users, or pull from database
//var theUser = config.NewUser(config.AllUserSettings())

var theUser = config.NewUser(&config.UserSettings{
	CanViewNumMedia:       true,
	CanViewMessages:       true,
	CanViewMessageFrom:    true,
	CanViewMessageTo:      true,
	CanViewMessageBody:    false,
	CanViewMessagePrice:   false,
	CanViewMedia:          true,
	CanViewCalls:          true,
	CanViewCallFrom:       true,
	CanViewCallTo:         true,
	CanViewCallPrice:      false,
	CanViewNumRecordings:  true,
	CanPlayRecordings:     true,
	CanViewRecordingPrice: false,
	CanViewConferences:    true,
	CanViewCallAlerts:     true,
})
