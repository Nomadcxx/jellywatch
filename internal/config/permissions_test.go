package config

import (
	"testing"
)

func TestPermissionsResolveUserGroup(t *testing.T) {
	// Use current user as a known valid username
	u := "nobody"
	p := &PermissionsConfig{User: u, Group: u}
	if _, err := p.ResolveUID(); err != nil {
		t.Fatalf("failed to resolve user %s: %v", u, err)
	}
	if _, err := p.ResolveGID(); err != nil {
		t.Fatalf("failed to resolve group %s: %v", u, err)
	}
}
