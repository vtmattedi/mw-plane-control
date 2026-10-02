package oidc

import (
	"testing"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
)

func TestAccessAllowedByApplicationPolicy(t *testing.T) {
	tests := []struct {
		name       string
		policy     string
		assignment *identityadmin.Assignment
		want       bool
	}{
		{name: "default deny without exception", policy: "default_deny", want: false},
		{name: "default allow without exception", policy: "default_allow", want: true},
		{name: "allow overrides default deny", policy: "default_deny", assignment: &identityadmin.Assignment{AccessEffect: "allow"}, want: true},
		{name: "deny overrides default allow", policy: "default_allow", assignment: &identityadmin.Assignment{AccessEffect: "deny"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := accessAllowedByApplicationPolicy(
				applications.Application{AccessPolicy: test.policy},
				test.assignment,
			)
			if got != test.want {
				t.Fatalf("allowed = %v; want %v", got, test.want)
			}
		})
	}
}
