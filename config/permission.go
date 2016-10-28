// Package config maintains information about permissions.
//
// The format and API's in this package will probably change over time.
package config

import (
	"errors"
	"time"
)

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
