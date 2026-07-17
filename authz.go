package main

import "strings"

// Authorizer resolves the Incus project a caller may act in. The project is the
// caller's group name with an optional prefix stripped, and all verbs are allowed
// within the caller's own project.
type Authorizer struct {
	groupPrefix string
}

func NewAuthorizer(groupPrefix string) *Authorizer {
	return &Authorizer{groupPrefix: groupPrefix}
}

// projectFor returns the project for the caller: the first group carrying the
// configured prefix, with the prefix stripped. ok is false when the caller has
// no such group.
func (a *Authorizer) projectFor(id Identity) (project string, ok bool) {
	for _, g := range id.Groups {
		if !strings.HasPrefix(g, a.groupPrefix) {
			continue
		}
		if p := strings.TrimPrefix(g, a.groupPrefix); p != "" {
			return p, true
		}
	}
	return "", false
}
