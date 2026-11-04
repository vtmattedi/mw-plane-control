package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
)

type ServiceConfig struct {
	SessionTTL   time.Duration
	PublicIssuer string
}

type LifecycleSigner interface {
	SignJWT(context.Context, map[string]any) (string, error)
}

type Service struct {
	deletionDispatchEnabled bool
	store                   Store
	audit                   audit.Recorder
	sessionTTL              time.Duration
	now                     func() time.Time
	dummyHash               string
	httpClient              *http.Client
	lifecycleSigner         LifecycleSigner
	publicIssuer            string
}

type LoginResult struct {
	User         User
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

type SessionResult struct {
	Session Session
	User    User
}

func NewService(
	store Store,
	auditRecorder audit.Recorder,
	cfg ServiceConfig,
) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("identity auth store is required")
	}
	if auditRecorder == nil {
		return nil, fmt.Errorf("identity audit recorder is required")
	}
	if cfg.SessionTTL <= 0 {
		return nil, fmt.Errorf("identity session TTL must be positive")
	}

	dummyHash, err := HashPassword(
		[]byte("mw-identity-dummy-password-never-used"),
	)
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}

	return &Service{
		deletionDispatchEnabled: true,
		store:                   store,
		audit:                   auditRecorder,
		sessionTTL:              cfg.SessionTTL,
		now:                     time.Now,
		dummyHash:               dummyHash,
		httpClient:              deletionHTTPClient(),
		publicIssuer:            strings.TrimRight(strings.TrimSpace(cfg.PublicIssuer), "/"),
	}, nil
}

func (s *Service) SetLifecycleSigner(signer LifecycleSigner) {
	s.lifecycleSigner = signer
}

func NormalizeEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	address, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(address.Address, value) {
		return "", fmt.Errorf("invalid email")
	}

	return strings.ToLower(address.Address), nil
}

func (s *Service) CreateSuperadmin(
	ctx context.Context,
	email string,
	displayName string,
	password []byte,
) (User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return User{}, err
	}

	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = normalized
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	userID, err := NewOpaqueToken("usr_")
	if err != nil {
		return User{}, err
	}

	now := s.now().UTC()
	input := NewUser{
		ID:              userID,
		Email:           strings.TrimSpace(email),
		EmailNormalized: normalized,
		DisplayName:     displayName,
		PasswordHash:    passwordHash,
		AccountType:     "workforce",
		Enabled:         true,
		IsSuperadmin:    true,
		CreatedAt:       now,
	}

	if err := s.store.CreateIdentityUser(ctx, input); err != nil {
		return User{}, err
	}

	user := User{
		ID:              input.ID,
		Email:           input.Email,
		EmailNormalized: input.EmailNormalized,
		DisplayName:     input.DisplayName,
		PasswordHash:    input.PasswordHash,
		AccountType:     input.AccountType,
		Enabled:         input.Enabled,
		IsSuperadmin:    input.IsSuperadmin,
		CreatedAt:       input.CreatedAt,
		UpdatedAt:       input.CreatedAt,
	}

	s.recordAudit(ctx, audit.Event{
		ID:         mustAuditID(),
		EventType:  "identity.user.created",
		Status:     "success",
		Severity:   "info",
		UserID:     &user.ID,
		ReasonCode: "bootstrap_superadmin",
		Metadata: map[string]any{
			"is_superadmin": true,
		},
		CreatedAt: now,
	})

	return user, nil
}

func (s *Service) VerifyPasswordForLogin(ctx context.Context, email string, password []byte, remoteIP string) (User, error) {
	normalized, normalizeErr := NormalizeEmail(email)
	if normalizeErr != nil {
		normalized = strings.ToLower(strings.TrimSpace(email))
	}

	user, err := s.store.IdentityUserByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			_, _ = VerifyPassword(password, s.dummyHash)
			s.recordLoginFailure(
				ctx,
				nil,
				remoteIP,
				"invalid_credentials",
				emailFingerprint(normalized),
			)
			return User{}, ErrInvalidLogin
		}
		return User{}, err
	}

	ok, verifyErr := VerifyPassword(password, user.PasswordHash)
	if verifyErr != nil {
		return User{}, fmt.Errorf(
			"verify stored password hash: %w",
			verifyErr,
		)
	}

	if !ok {
		s.recordLoginFailure(
			ctx,
			&user.ID,
			remoteIP,
			"invalid_credentials",
			"",
		)
		return User{}, ErrInvalidLogin
	}

	if !user.Enabled || user.DeletionRequestedAt != nil {
		s.recordLoginFailure(
			ctx,
			&user.ID,
			remoteIP,
			"user_disabled",
			"",
		)
		return User{}, ErrUserDisabled
	}

	pending, err := s.store.IdentityUserRecoveryPending(ctx, user.ID)
	if err != nil {
		return User{}, err
	}
	if pending {
		return User{}, ErrRecoveryPending
	}

	return user, nil
}
func (s *Service) Login(
	ctx context.Context,
	email string,
	password []byte,
	remoteIP string,
) (LoginResult, error) {
	user, err := s.VerifyPasswordForLogin(ctx, email, password, remoteIP)
	if err != nil {
		return LoginResult{}, err
	}
	sessionToken, err := NewOpaqueToken("mwid_")
	if err != nil {
		return LoginResult{}, err
	}

	csrfToken, err := NewOpaqueToken("mwcsrf_")
	if err != nil {
		return LoginResult{}, err
	}

	sessionID, err := NewOpaqueToken("sid_")
	if err != nil {
		return LoginResult{}, err
	}

	now := s.now().UTC()
	expiresAt := now.Add(s.sessionTTL)

	if err := s.store.CreateIdentitySession(
		ctx,
		SessionRecord{
			ID:         sessionID,
			UserID:     user.ID,
			TokenHash:  HashToken(sessionToken),
			CSRFHash:   HashToken(csrfToken),
			AuthMethod: "password",
			Proof:      ProofPassword,
			AuthTime:   now,
			ExpiresAt:  expiresAt,
			CreatedAt:  now,
			LastSeenAt: now,
		},
	); err != nil {
		return LoginResult{}, err
	}

	_ = s.store.DeleteExpiredIdentitySessions(ctx, now)

	s.recordAudit(ctx, audit.Event{
		ID:         mustAuditID(),
		EventType:  "identity.login.completed",
		Status:     "success",
		Severity:   "info",
		UserID:     &user.ID,
		RemoteIP:   remoteIP,
		ReasonCode: "password",
		CreatedAt:  now,
	})

	return LoginResult{
		User:         user,
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *Service) CreateSessionForUser(
	ctx context.Context,
	userID string,
	remoteIP string,
	reason string,
) (LoginResult, error) {
	user, err := s.store.IdentityUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LoginResult{}, ErrInvalidLogin
		}
		return LoginResult{}, err
	}
	if !user.Enabled || user.DeletionRequestedAt != nil {
		return LoginResult{}, ErrUserDisabled
	}
	pending, err := s.store.IdentityUserRecoveryPending(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if pending {
		return LoginResult{}, ErrRecoveryPending
	}

	sessionToken, err := NewOpaqueToken("mwid_")
	if err != nil {
		return LoginResult{}, err
	}
	csrfToken, err := NewOpaqueToken("mwcsrf_")
	if err != nil {
		return LoginResult{}, err
	}
	sessionID, err := NewOpaqueToken("sid_")
	if err != nil {
		return LoginResult{}, err
	}

	now := s.now().UTC()
	expiresAt := now.Add(s.sessionTTL)
	if err := s.store.CreateIdentitySession(
		ctx,
		SessionRecord{
			ID:         sessionID,
			UserID:     user.ID,
			TokenHash:  HashToken(sessionToken),
			CSRFHash:   HashToken(csrfToken),
			AuthMethod: authMethodFromReason(reason),
			Proof:      proofFromReason(reason),
			AuthTime:   now,
			ExpiresAt:  expiresAt,
			CreatedAt:  now,
			LastSeenAt: now,
		},
	); err != nil {
		return LoginResult{}, err
	}

	_ = s.store.DeleteExpiredIdentitySessions(ctx, now)
	if strings.TrimSpace(reason) == "" {
		reason = "authenticated"
	}
	s.recordAudit(ctx, audit.Event{
		ID:         mustAuditID(),
		EventType:  "identity.login.completed",
		Status:     "success",
		Severity:   "info",
		UserID:     &user.ID,
		RemoteIP:   remoteIP,
		ReasonCode: reason,
		CreatedAt:  now,
	})

	return LoginResult{
		User:         user,
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
		ExpiresAt:    expiresAt,
	}, nil
}

func authMethodFromReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "webauthn":
		return "webauthn"
	case "password", "public_registration":
		return "password"
	case "password_totp":
		return "password_totp"
	case "totp":
		return "totp"
	case "recovery":
		return "recovery"
	default:
		return "unknown"
	}
}

func (s *Service) Session(
	ctx context.Context,
	rawToken string,
) (SessionResult, error) {
	if strings.TrimSpace(rawToken) == "" {
		return SessionResult{}, ErrInvalidSession
	}

	now := s.now().UTC()
	session, err := s.store.IdentitySessionByTokenHash(
		ctx,
		HashToken(rawToken),
		now,
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SessionResult{}, ErrInvalidSession
		}
		return SessionResult{}, err
	}

	if !session.User.Enabled || session.User.DeletionRequestedAt != nil {
		return SessionResult{}, ErrInvalidSession
	}

	return SessionResult{
		Session: session,
		User:    session.User,
	}, nil
}

func (s *Service) RotateCSRF(
	ctx context.Context,
	rawSessionToken string,
) (string, error) {
	csrfToken, err := NewOpaqueToken("mwcsrf_")
	if err != nil {
		return "", err
	}

	if err := s.store.UpdateIdentitySessionCSRF(
		ctx,
		HashToken(rawSessionToken),
		HashToken(csrfToken),
	); err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrInvalidSession
		}
		return "", err
	}

	return csrfToken, nil
}

func (s *Service) Logout(
	ctx context.Context,
	rawSessionToken string,
	remoteIP string,
) error {
	if strings.TrimSpace(rawSessionToken) == "" {
		return nil
	}

	session, err := s.Session(ctx, rawSessionToken)
	if err != nil && !errors.Is(err, ErrInvalidSession) {
		return err
	}

	if deleteErr := s.store.DeleteIdentitySessionByTokenHash(
		ctx,
		HashToken(rawSessionToken),
	); deleteErr != nil {
		return deleteErr
	}

	if session.User.ID != "" {
		s.recordAudit(ctx, audit.Event{
			ID:         mustAuditID(),
			EventType:  "identity.logout.completed",
			Status:     "success",
			Severity:   "info",
			UserID:     &session.User.ID,
			RemoteIP:   remoteIP,
			ReasonCode: "user_logout",
			CreatedAt:  s.now().UTC(),
		})
	}

	return nil
}

func (s *Service) recordLoginFailure(
	ctx context.Context,
	userID *string,
	remoteIP string,
	reason string,
	emailHash string,
) {
	metadata := map[string]any{}
	if emailHash != "" {
		metadata["email_hash_prefix"] = emailHash
	}

	s.recordAudit(ctx, audit.Event{
		ID:         mustAuditID(),
		EventType:  "identity.login.failed",
		Status:     "failure",
		Severity:   "warning",
		UserID:     userID,
		RemoteIP:   remoteIP,
		ReasonCode: reason,
		Metadata:   metadata,
		CreatedAt:  s.now().UTC(),
	})
}

func (s *Service) recordAudit(
	ctx context.Context,
	event audit.Event,
) {
	if err := s.audit.RecordAuditEvent(ctx, event); err != nil {
		slog.Error(
			"identity audit write failed",
			"event_type", event.EventType,
			"error", err,
		)
	}
}

func mustAuditID() string {
	value, err := NewOpaqueToken("aud_")
	if err != nil {
		panic(err)
	}
	return value
}

func emailFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func proofFromReason(reason string) string {
	switch reason {
	case "webauthn":
		return ProofPasskey
	case "password_totp":
		return ProofMFA
	case "public_registration", "password":
		return ProofPassword
	}
	return ""
}
