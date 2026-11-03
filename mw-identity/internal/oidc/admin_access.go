package oidc

import (
	"context"
	"errors"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
)

const IdentityAdminAudience = "mw-identity-admin"

type AccessContext struct {
	Token       AccessToken
	User        auth.User
	Application applications.Application
}

func (s *Service) ValidateAccessToken(
	ctx context.Context,
	rawAccessToken string,
	requiredAudience string,
	requiredScope string,
) (AccessContext, *ProtocolError) {
	rawAccessToken = strings.TrimSpace(
		rawAccessToken,
	)
	if rawAccessToken == "" {
		return AccessContext{}, invalidToken()
	}

	token, err := s.store.AccessTokenByHash(
		ctx,
		hashOpaque(rawAccessToken),
		s.now().UTC(),
	)
	if err != nil {
		return AccessContext{}, invalidToken()
	}

	if requiredAudience != "" &&
		token.Audience != requiredAudience {
		return AccessContext{}, invalidToken()
	}
	if requiredScope != "" &&
		!contains(token.Scopes, requiredScope) {
		return AccessContext{}, invalidToken()
	}

	user, err := s.store.IdentityUserByID(
		ctx,
		token.UserID,
	)
	if err != nil || !user.Enabled {
		return AccessContext{}, invalidToken()
	}

	app, err := s.store.ApplicationByClientID(
		ctx,
		token.ClientID,
	)
	if err != nil ||
		!app.Enabled ||
		app.ArchivedAt != nil {
		return AccessContext{}, invalidToken()
	}

	return AccessContext{
		Token:       token,
		User:        user,
		Application: app,
	}, nil
}

func (s *Service) AuthorizeIdentityUser(
	ctx context.Context,
	authorization ValidatedAuthorization,
	user auth.User,
) *ProtocolError {
	if !user.Enabled || user.DeletionRequestedAt != nil || !authorization.Application.Enabled || authorization.Application.ArchivedAt != nil {
		return accessDenied()
	}
	if user.AccountType == "public" && contains(
		authorization.Application.Audiences,
		IdentityAdminAudience,
	) {
		return accessDenied()
	}
	if user.IsSuperadmin &&
		contains(
			authorization.Scopes,
			"identity.admin",
		) &&
		contains(
			authorization.Application.Audiences,
			IdentityAdminAudience,
		) {
		return nil
	}
	// Preserve the explicitly unmigrated legacy exception only. Ordinary v1
	// clients require their own access assignment even for a global superadmin.
	if !authorization.Application.AccessPolicyV1 && user.IsSuperadmin && !contains(
		authorization.Application.Audiences,
		IdentityAdminAudience,
	) {
		return nil
	}

	assignment, err := s.store.IdentityAssignment(
		ctx,
		authorization.Application.ID,
		user.ID,
	)
	if err == nil {
		if accessAllowedByApplicationPolicy(
			authorization.Application,
			&assignment,
		) {
			return nil
		}
		return accessDenied()
	}
	if errors.Is(err, identityadmin.ErrNotFound) {
		if accessAllowedByApplicationPolicy(
			authorization.Application,
			nil,
		) {
			return nil
		}
		return accessDenied()
	}

	return serverError(
		"could not evaluate application assignment",
	)
}

func accessAllowedByApplicationPolicy(
	application applications.Application,
	assignment *identityadmin.Assignment,
) bool {
	if assignment != nil {
		return assignment.AccessEffect == "allow"
	}
	return application.AccessPolicy == "default_allow" || (!application.AccessPolicyV1 && application.AllowPublicSignup && !application.Internal && !contains(application.Audiences, IdentityAdminAudience))
}

func accessDenied() *ProtocolError {
	return &ProtocolError{
		Code:            "access_denied",
		Description:     "identity is denied by the application access policy",
		HTTPStatus:      403,
		RedirectAllowed: true,
	}
}
