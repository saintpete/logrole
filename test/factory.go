package test

import log "github.com/inconshreveable/log15"

var NullLogger = log.New()

func init() {
	NullLogger.SetHandler(log.DiscardHandler())
}
