package main

import (
	"net/http"
	"strings"
)

// Identity is what the authentication proxy tells us about the caller via
// X-Auth-* headers. The provisioner trusts these and does not authenticate.
type Identity struct {
	Email    string
	Username string
	Groups   []string
}

func identityFromHeaders(r *http.Request) Identity {
	id := Identity{
		Email:    r.Header.Get("X-Auth-Email"),
		Username: r.Header.Get("X-Auth-Preferred-Username"),
	}
	if g := r.Header.Get("X-Auth-Groups"); g != "" {
		for _, p := range strings.Split(g, ",") {
			if p = strings.TrimSpace(p); p != "" {
				id.Groups = append(id.Groups, p)
			}
		}
	}
	return id
}

func (i Identity) hasGroup(g string) bool {
	for _, x := range i.Groups {
		if x == g {
			return true
		}
	}
	return false
}
