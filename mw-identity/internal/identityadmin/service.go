package identityadmin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

type Service struct {
	production   bool
	store        Store
	applications *applications.Service
	now          func() time.Time
}

type CreateApplicationRequest struct {
	Name                   string
	ClientID               string
	ApplicationType        string
	AuthenticationPolicy   string
	AccessPolicy           string
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	DeleteCallbackURI      string
	AccountManagementURI   string
	AccountDeletionURI     string
	AllowPublicSignup      bool
	Internal               bool
	Scopes                 []string
	Claims                 []string
	Audiences              []string
}

type CreateApplicationResult struct {
	Application  applications.Application
	ClientSecret string
}

type RotateSecretResult struct {
	ClientSecret string
}

func New(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf(
			"Identity admin store is required",
		)
	}

	appService, err := applications.NewService(store)
	if err != nil {
		return nil, err
	}

	return &Service{
		store:        store,
		applications: appService,
		now:          time.Now,
	}, nil
}

func (s *Service) Permissions(
	ctx context.Context,
	user auth.User,
	clientID string,
) ([]string, string, error) {
	if !user.Enabled {
		return nil, "", ErrRoleForbidden
	}
	if user.AccountType == "public" {
		return nil, "", ErrRoleForbidden
	}

	role := ""
	if user.IsSuperadmin {
		role = "superadmin"
	} else {
		value, err :=
			s.store.IdentityAssignmentRoleByClientID(
				ctx,
				clientID,
				user.ID,
			)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, "", ErrRoleForbidden
			}
			return nil, "", err
		}
		role = value
	}

	switch role {
	case "superadmin":
		return []string{
			PermissionApplicationsRead,
			PermissionApplicationsWrite,
			PermissionApplicationsRotate,
			PermissionUsersRead,
			PermissionUsersWrite,
			PermissionAssignmentsRead,
			PermissionAssignmentsWrite,
			PermissionAuditRead,
		}, role, nil

	case "admin":
		return []string{
			PermissionApplicationsRead,
			PermissionApplicationsWrite,
			PermissionApplicationsRotate,
			PermissionUsersRead,
			PermissionUsersWrite,
			PermissionAssignmentsRead,
			PermissionAuditRead,
		}, role, nil

	default:
		return nil, role, ErrRoleForbidden
	}
}

func HasPermission(
	permissions []string,
	required string,
) bool {
	for _, permission := range permissions {
		if permission == required {
			return true
		}
	}
	return false
}

func (s *Service) ListApplications(
	ctx context.Context,
) ([]applications.Application, error) {
	return s.store.ListIdentityApplications(ctx)
}

func (s *Service) Application(
	ctx context.Context,
	id string,
) (applications.Application, error) {
	return s.store.IdentityApplicationByID(ctx, id)
}

func (s *Service) CreateApplication(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	request CreateApplicationRequest,
) (CreateApplicationResult, error) {
	if request.Internal || privilegedRegistration(request.Scopes, request.Audiences) {
		return CreateApplicationResult{}, ErrRoleForbidden
	}
	if (strings.TrimSpace(request.AuthenticationPolicy) != "" && strings.TrimSpace(request.AuthenticationPolicy) != "standard") || request.AllowPublicSignup {
		return CreateApplicationResult{}, ErrInvalid
	}
	result, err := s.applications.Create(
		ctx,
		applications.CreateRequest{
			Name:                   request.Name,
			ClientID:               request.ClientID,
			ApplicationType:        request.ApplicationType,
			AuthenticationPolicy:   request.AuthenticationPolicy,
			AccessPolicy:           request.AccessPolicy,
			RedirectURIs:           request.RedirectURIs,
			PostLogoutRedirectURIs: request.PostLogoutRedirectURIs,
			DeleteCallbackURI:      request.DeleteCallbackURI,
			AccountManagementURI:   request.AccountManagementURI,
			AccountDeletionURI:     request.AccountDeletionURI,
			AllowPublicSignup:      request.AllowPublicSignup,
			Internal:               request.Internal,
			Scopes:                 request.Scopes,
			Claims:                 request.Claims,
			Audiences:              request.Audiences,
		},
	)
	if err != nil {
		return CreateApplicationResult{}, err
	}

	_ = s.recordAudit(
		ctx,
		actor,
		remoteIP,
		"identity.application.created",
		map[string]any{
			"application_id": result.Application.ID,
			"client_id":      result.Application.ClientID,
			"internal":       result.Application.Internal,
		},
	)

	return CreateApplicationResult{
		Application:  result.Application,
		ClientSecret: result.ClientSecret,
	}, nil
}

func (s *Service) CreateUser(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	request CreateUserRequest,
) (UserSummary, error) {
	normalized, err := auth.NormalizeEmail(request.Email)
	if err != nil || len(request.Password) < 12 || len(request.Password) > 256 {
		return UserSummary{}, ErrInvalid
	}

	displayName := strings.TrimSpace(request.DisplayName)
	if displayName == "" {
		displayName = normalized
	}
	passwordHash, err := auth.HashPassword([]byte(request.Password))
	if err != nil {
		return UserSummary{}, err
	}
	userID, err := auth.NewOpaqueToken("usr_")
	if err != nil {
		return UserSummary{}, err
	}
	now := s.now().UTC()
	input := auth.NewUser{
		ID:              userID,
		Email:           strings.TrimSpace(request.Email),
		EmailNormalized: normalized,
		DisplayName:     displayName,
		PasswordHash:    passwordHash,
		AccountType:     "workforce",
		Enabled:         true,
		IsSuperadmin:    false,
		CreatedAt:       now,
	}
	if err := s.store.CreateIdentityUser(ctx, input); err != nil {
		return UserSummary{}, err
	}

	_ = s.recordAudit(ctx, actor, remoteIP, "identity.user.created", map[string]any{
		"user_id":       userID,
		"email":         normalized,
		"is_superadmin": false,
	})

	return UserSummary{
		ID: userID, Email: input.Email, DisplayName: displayName,
		Enabled: true, IsSuperadmin: false, CreatedAt: now,
		AccountType: "workforce",
	}, nil
}

func (s *Service) UpdateApplication(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	id string,
	request UpdateApplication,
) (applications.Application, error) {
	current, err := s.store.IdentityApplicationByID(
		ctx,
		id,
	)
	if err != nil {
		return applications.Application{}, err
	}
	if current.ArchivedAt != nil {
		return applications.Application{}, ErrArchived
	}

	if current.Internal || request.Internal != current.Internal || privilegedRegistration(request.Scopes, request.Audiences) {
		return applications.Application{}, ErrRoleForbidden
	}
	if request.AuthenticationPolicy != "" && request.AuthenticationPolicy != current.AuthenticationPolicy {
		return applications.Application{}, ErrInvalid
	}
	request.AuthenticationPolicy = current.AuthenticationPolicy
	request.AllowPublicSignup = current.AllowPublicSignup
	normalized, err := normalizeUpdate(request)
	if err != nil {
		return applications.Application{}, err
	}

	if s.production {
		uris := append(append([]string{}, normalized.RedirectURIs...), normalized.PostLogoutRedirectURIs...)
		uris = append(uris, normalized.DeleteCallbackURI, normalized.AccountManagementURI, normalized.AccountDeletionURI)
		if err = applications.ValidateProductionURIs(uris...); err != nil {
			return applications.Application{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	if err := s.store.UpdateIdentityApplication(
		ctx,
		id,
		normalized,
		now,
	); err != nil {
		return applications.Application{}, err
	}

	updated, err := s.store.IdentityApplicationByID(
		ctx,
		id,
	)
	if err != nil {
		return applications.Application{}, err
	}

	_ = s.recordAudit(
		ctx,
		actor,
		remoteIP,
		"identity.application.updated",
		map[string]any{
			"application_id": id,
			"client_id":      updated.ClientID,
			"internal":       updated.Internal,
		},
	)

	return updated, nil
}

func (s *Service) SetApplicationState(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	id string,
	action string,
) (applications.Application, error) {
	current, err := s.store.IdentityApplicationByID(
		ctx,
		id,
	)
	if err != nil {
		return applications.Application{}, err
	}

	now := s.now().UTC()
	var (
		enabled  bool
		archived *time.Time
		event    string
	)

	switch action {
	case "disable":
		enabled = false
		archived = current.ArchivedAt
		event = "identity.application.disabled"

	case "enable":
		if current.ArchivedAt != nil {
			return applications.Application{}, ErrArchived
		}
		enabled = true
		event = "identity.application.enabled"

	case "archive":
		enabled = false
		value := now
		archived = &value
		event = "identity.application.archived"

	default:
		return applications.Application{}, ErrInvalid
	}

	if err := s.store.SetIdentityApplicationState(
		ctx,
		id,
		enabled,
		archived,
		now,
	); err != nil {
		return applications.Application{}, err
	}

	updated, err := s.store.IdentityApplicationByID(
		ctx,
		id,
	)
	if err != nil {
		return applications.Application{}, err
	}

	_ = s.recordAudit(
		ctx,
		actor,
		remoteIP,
		event,
		map[string]any{
			"application_id": id,
			"client_id":      updated.ClientID,
		},
	)

	return updated, nil
}

func (s *Service) DeleteApplication(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	id string,
) error {
	app, err := s.store.IdentityApplicationByID(ctx, id)
	if err != nil {
		return err
	}
	if app.ArchivedAt == nil {
		return ErrArchiveRequired
	}

	if err := s.store.PurgeIdentityApplication(ctx, id, app.ClientID); err != nil {
		return err
	}

	_ = s.recordAudit(
		ctx,
		actor,
		remoteIP,
		"identity.application.deleted",
		map[string]any{
			"application_id": id,
			"client_id":      app.ClientID,
		},
	)

	return nil
}

func (s *Service) RotateSecret(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	id string,
) (RotateSecretResult, error) {
	app, err := s.store.IdentityApplicationByID(
		ctx,
		id,
	)
	if err != nil {
		return RotateSecretResult{}, err
	}
	if app.ArchivedAt != nil {
		return RotateSecretResult{}, ErrArchived
	}
	if app.ApplicationType != "confidential" {
		return RotateSecretResult{}, ErrInvalid
	}

	rawSecret, err := randomSecret()
	if err != nil {
		return RotateSecretResult{}, err
	}

	if err := s.store.UpdateIdentityApplicationSecretHash(
		ctx,
		id,
		applications.HashClientSecret(rawSecret),
		s.now().UTC(),
	); err != nil {
		return RotateSecretResult{}, err
	}

	_ = s.recordAudit(
		ctx,
		actor,
		remoteIP,
		"identity.application.secret_rotated",
		map[string]any{
			"application_id": id,
			"client_id":      app.ClientID,
		},
	)

	return RotateSecretResult{
		ClientSecret: rawSecret,
	}, nil
}

func (s *Service) Users(
	ctx context.Context,
) ([]UserSummary, error) {
	return s.store.ListIdentityUsers(ctx)
}

func (s *Service) Assignments(
	ctx context.Context,
	applicationID string,
) ([]Assignment, error) {
	if _, err := s.store.IdentityApplicationByID(
		ctx,
		applicationID,
	); err != nil {
		return nil, err
	}

	return s.store.ListIdentityAssignments(
		ctx,
		applicationID,
	)
}

func (s *Service) UpsertAssignment(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	applicationID string,
	userID string,
	role string,
	accessEffect string,
) (Assignment, error) {
	role = strings.TrimSpace(role)
	accessEffect = strings.TrimSpace(accessEffect)
	if role != "user" &&
		role != "admin" &&
		role != "superadmin" {
		return Assignment{}, ErrInvalid
	}
	if accessEffect == "" {
		accessEffect = "allow"
	}
	if accessEffect != "allow" && accessEffect != "deny" {
		return Assignment{}, ErrInvalid
	}

	app, err := s.store.IdentityApplicationByID(
		ctx,
		applicationID,
	)
	if err != nil {
		return Assignment{}, err
	}
	user, err := s.store.IdentityUserByID(
		ctx,
		userID,
	)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return Assignment{}, ErrNotFound
		}
		return Assignment{}, err
	}
	// Ordinary application roles do not override explicit access denials.
	// Preserve lockout protections only for the MW administration audience.
	if accessEffect == "deny" && contains(app.Audiences, "mw-identity-admin") {
		if user.IsSuperadmin || role == "admin" || role == "superadmin" {
			return Assignment{}, ErrRoleForbidden
		}
		existing, existingErr := s.store.IdentityAssignment(ctx, applicationID, userID)
		if existingErr == nil && (existing.Role == "admin" || existing.Role == "superadmin") {
			return Assignment{}, ErrRoleForbidden
		}
		if existingErr != nil && !errors.Is(existingErr, ErrNotFound) {
			return Assignment{}, existingErr
		}
	}

	now := s.now().UTC()
	assignment := Assignment{
		ApplicationID: applicationID,
		UserID:        userID,
		Role:          role,
		AccessEffect:  accessEffect,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.store.UpsertIdentityAssignment(
		ctx,
		assignment,
	); err != nil {
		return Assignment{}, err
	}

	_ = s.recordAudit(
		ctx,
		actor,
		remoteIP,
		"identity.assignment.upserted",
		map[string]any{
			"application_id": applicationID,
			"user_id":        userID,
			"role":           role,
			"access_effect":  accessEffect,
		},
	)

	return assignment, nil
}

func (s *Service) DeleteAssignment(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	applicationID string,
	userID string,
) error {
	if err := s.store.DeleteIdentityAssignment(
		ctx,
		applicationID,
		userID,
	); err != nil {
		return err
	}

	_ = s.recordAudit(
		ctx,
		actor,
		remoteIP,
		"identity.assignment.deleted",
		map[string]any{
			"application_id": applicationID,
			"user_id":        userID,
		},
	)

	return nil
}

func (s *Service) Audit(
	ctx context.Context,
	limit int,
) ([]AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	return s.store.ListIdentityAudit(
		ctx,
		limit,
	)
}

func (s *Service) recordAudit(
	ctx context.Context,
	actor auth.User,
	remoteIP string,
	eventType string,
	metadata map[string]any,
) error {
	id, err := randomID("audit_")
	if err != nil {
		return err
	}

	userID := actor.ID
	return s.store.RecordAuditEvent(
		ctx,
		audit.Event{
			ID:        id,
			EventType: eventType,
			Status:    "success",
			Severity:  "info",
			UserID:    &userID,
			RemoteIP:  remoteIP,
			Metadata:  metadata,
			CreatedAt: s.now().UTC(),
		},
	)
}

func normalizeUpdate(
	request UpdateApplication,
) (UpdateApplication, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.AuthenticationPolicy =
		strings.TrimSpace(
			request.AuthenticationPolicy,
		)
	request.AccessPolicy = strings.TrimSpace(request.AccessPolicy)

	if request.Name == "" {
		return UpdateApplication{}, ErrInvalid
	}
	if request.AuthenticationPolicy == "" {
		request.AuthenticationPolicy = "standard"
	}
	if request.AuthenticationPolicy != "standard" && request.AuthenticationPolicy != "strong" {
		return UpdateApplication{}, ErrInvalid
	}
	if request.AccessPolicy == "" {
		request.AccessPolicy = "default_deny"
	}
	if request.AccessPolicy != "default_allow" && request.AccessPolicy != "default_deny" {
		return UpdateApplication{}, ErrInvalid
	}

	var err error
	request.AccountManagementURI, err = applications.ValidateNavigationURI(request.AccountManagementURI)
	if err != nil {
		return UpdateApplication{}, ErrInvalid
	}
	request.AccountDeletionURI, err = applications.ValidateNavigationURI(request.AccountDeletionURI)
	if err != nil {
		return UpdateApplication{}, ErrInvalid
	}
	request.RedirectURIs, err = normalizeURIs(
		request.RedirectURIs,
		true,
	)
	if err != nil || len(request.RedirectURIs) == 0 {
		return UpdateApplication{}, ErrInvalid
	}

	request.PostLogoutRedirectURIs, err =
		normalizeURIs(
			request.PostLogoutRedirectURIs,
			false,
		)
	if err != nil {
		return UpdateApplication{}, ErrInvalid
	}
	callbacks, err := normalizeURIs([]string{request.DeleteCallbackURI}, true)
	if strings.TrimSpace(request.DeleteCallbackURI) == "" {
		request.DeleteCallbackURI = ""
	} else if err != nil || len(callbacks) != 1 {
		return UpdateApplication{}, ErrInvalid
	} else {
		request.DeleteCallbackURI = callbacks[0]
	}
	// An application registered before the callback was mandatory has to add
	// one on its next edit; enabling, disabling and archiving stay available.
	if err := applications.CheckLifecycle(request.Internal, request.AllowPublicSignup, request.DeleteCallbackURI); err != nil {
		return UpdateApplication{}, err
	}

	request.Scopes = normalizeValues(
		request.Scopes,
	)
	if !contains(request.Scopes, "openid") {
		return UpdateApplication{}, ErrInvalid
	}

	request.Claims = normalizeValues(
		request.Claims,
	)
	request.Audiences = normalizeValues(
		request.Audiences,
	)

	return request, nil
}

func normalizeURIs(
	values []string,
	requireHTTPS bool,
) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		parsed, err := url.Parse(value)
		if err != nil ||
			parsed.Scheme == "" ||
			parsed.Host == "" || len(value) > 2048 ||
			parsed.User != nil ||
			parsed.Fragment != "" {
			return nil, ErrInvalid
		}

		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && parsed.Hostname() == "localhost") {
			return nil, ErrInvalid
		}

		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	sort.Strings(result)
	return result, nil
}

func normalizeValues(
	values []string,
) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	sort.Strings(result)
	return result
}

func contains(
	values []string,
	needle string,
) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func randomSecret() (string, error) {
	value, err := randomID("mw_client_")
	if err != nil {
		return "", err
	}
	return value, nil
}

func randomID(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf(
			"generate Identity admin identifier: %w",
			err,
		)
	}

	return prefix +
		base64.RawURLEncoding.EncodeToString(raw), nil
}

func privilegedRegistration(scopes, audiences []string) bool {
	for _, v := range scopes {
		if strings.HasPrefix(strings.TrimSpace(v), "identity.") {
			return true
		}
	}
	for _, v := range audiences {
		if strings.TrimSpace(v) == "mw-identity-admin" {
			return true
		}
	}
	return false
}

func (s *Service) RequireHTTPS(production bool) {
	s.production = production
	s.applications.RequireHTTPS(production)
}
