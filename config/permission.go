// Package config maintains information about permissions.
//
// The format and API's in this package will probably change over time.
package config

import (
	"errors"
	"time"
)

const DefaultPort = "4114"
const DefaultPageSize = 50

// DefaultMaxResourceAge allows all resources to be fetched. The company was
// founded in 2008, so there should definitely be no resources created in the
// 1980's.
var DefaultMaxResourceAge = time.Since(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))

type Permission struct {
	maxResourceAge time.Duration
}

// ErrTooOld is returned for a resource that's more than MaxResourceAge old.
var ErrTooOld = errors.New("Cannot access this resource because its age exceeds the viewable limit")
var PermissionDenied = errors.New("You do not have permission to access that information")

func (p *Permission) MaxResourceAge() time.Duration {
	return p.maxResourceAge
}

func NewPermission(maxResourceAge time.Duration) *Permission {
	return &Permission{
		maxResourceAge: maxResourceAge,
	}
}
