package services

import (
	"os"
	"strings"
)

type Rlm string

const Prod = Rlm("prod")
const Local = Rlm("local")

// Realm returns the given Rlm. If the realm cannot be determined, Local is
// returned.
func Realm() Rlm {
	env, ok := os.LookupEnv("REALM")
	if !ok {
		return Local
	}
	env = strings.ToLower(env)
	switch env {
	case "prod":
		return Prod
	case "local":
		return Local
	default:
		return Local
	}
}
