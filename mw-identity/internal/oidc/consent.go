package oidc

import (
	"context"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

type Consent struct {
	Version int64
	Active  bool
	Scopes  []string
}

func (s *Service) Consent(ctx context.Context, a ValidatedAuthorization, user auth.User) (Consent, error) {
	c, err := s.store.IdentityConsent(ctx, user.ID, a.Application.ID)
	if err == nil && a.Application.Internal {
		if c.Version == 0 {
			_ = s.store.GrantIdentityConsent(ctx, user.ID, a.Application.ID, a.Application.Scopes, 0, s.now().UTC())
			return s.store.IdentityConsent(ctx, user.ID, a.Application.ID)
		}
		if c.Active {
			c.Scopes = a.Application.Scopes
		}
	}
	return c, err
}

func ConsentCovers(c Consent, scopes []string) bool {
	if !c.Active {
		return false
	}
	if c.Version <= 0 {
		return false
	}
	for _, scope := range scopes {
		if !contains(c.Scopes, scope) {
			return false
		}
	}
	return true
}

func (s *Service) GrantConsent(ctx context.Context, a ValidatedAuthorization, user auth.User, expected int64) error {
	if e := s.AuthorizeIdentityUser(ctx, a, user); e != nil {
		return e
	}
	return s.store.GrantIdentityConsent(ctx, user.ID, a.Application.ID, a.Scopes, expected, time.Now().UTC())
}
