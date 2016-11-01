// Package config maintains information about permissions.
//
// The format and API's in this package will probably change over time.
package config

import (
	"errors"
	"fmt"
	"time"
)

type Group struct {
	Permissions *UserSettings `yaml:"permissions"`
	Name        string        `yaml:"name"`
	Default     bool          `yaml:"default,omitempty"`
	Users       []string      `yaml:"users"`
}

// TODO naming here
type Policy []*Group

// Lookup finds the User with the given id. If no user with that name is found,
// but a default group is defined, a user from that group is returned. The
// boolean is true if a user was found directly by id. Otherwise returns an
// error.
//
// Lookup assumes the Policy is valid.
func (p *Policy) Lookup(id string) (*User, bool, error) {
	if p == nil {
		return nil, false, errors.New("nil policy")
	}
	var defaultGroup *Group
	for _, group := range *p {
		for _, user := range group.Users {
			if user == id {
				return NewUser(group.Permissions), true, nil
			}
		}
		if group.Default == true {
			defaultGroup = group
		}
	}
	if defaultGroup != nil {
		return NewUser(defaultGroup.Permissions), false, nil
	}
	return nil, false, fmt.Errorf("User %s not found in the policy, and no default configured", id)
}

// Users returns a map of all Users defined in the policy. Users assumes the
// Policy is valid.
func (p *Policy) Users() map[string]*User {
	users := make(map[string]*User)
	if p == nil {
		return users
	}
	for _, group := range *p {
		for _, user := range group.Users {
			users[user] = NewUser(group.Permissions)
		}
	}
	return users
}

type Permission struct {
	maxResourceAge time.Duration
}

func validatePolicy(p *Policy) error {
	if p == nil {
		return nil
	}
	users := make(map[string]bool)
	names := make(map[string]bool)
	defaultCount := 0
	for _, group := range *p {
		if group.Name == "" {
			return errors.New("Group has no name, define a group name")
		}
		if _, ok := names[group.Name]; ok {
			return fmt.Errorf("Group name %s appears twice in the list", group.Name)
		}
		names[group.Name] = true
		if group.Default == true {
			defaultCount++
			if defaultCount > 1 {
				return errors.New("More than one group marked as default")
			}
		}
		for _, user := range group.Users {
			if _, ok := users[user]; ok {
				// Open to allowing this later, but for now this just
				// complicates the permission model.
				return fmt.Errorf("User %s appears twice in the list", user)
			}
			users[user] = true
		}
	}
	return nil
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
