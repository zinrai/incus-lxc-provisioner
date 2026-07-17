package main

import "testing"

// The group->project convention is the tenant boundary: if it resolves the wrong
// project, a caller acts in someone else's tenancy. So it gets a test.
func TestProjectFor(t *testing.T) {
	cases := []struct {
		name    string
		prefix  string
		groups  []string
		project string
		ok      bool
	}{
		{"group is the project", "", []string{"team-a"}, "team-a", true},
		{"first group wins", "", []string{"team-a", "team-b"}, "team-a", true},
		{"no group", "", nil, "", false},
		{"prefix is stripped", "tenant-", []string{"tenant-team-a"}, "team-a", true},
		{"prefix skips non-tenant groups", "tenant-", []string{"staff", "tenant-team-b"}, "team-b", true},
		{"prefix with no tenant group", "tenant-", []string{"staff"}, "", false},
		{"prefix-only group is not a project", "tenant-", []string{"tenant-"}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			project, ok := NewAuthorizer(c.prefix).projectFor(Identity{Groups: c.groups})
			if ok != c.ok || project != c.project {
				t.Fatalf("projectFor(prefix=%q, groups=%v) = (%q, %v); want (%q, %v)",
					c.prefix, c.groups, project, ok, c.project, c.ok)
			}
		})
	}
}
