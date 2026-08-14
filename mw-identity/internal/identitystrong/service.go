package identitystrong

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

type Options struct {
	SecretKeys       string
	ActiveKeyVersion int
	Issuer           string
	RecoveryTTL      time.Duration
}

type Service struct {
	store       Store
	keyring     *Keyring
	issuer      string
	recoveryTTL time.Duration
	now         func() time.Time
}

type TOTPSetup struct {
	Secret     string `json:"secret"`
	OTPAuthURI string `json:"otpauth_uri"`
}

type RecoveryStarted struct {
	User      auth.User
	Token     string
	ExpiresAt time.Time
}

func NewService(store Store, options Options) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("strong-auth store is required")
	}
	keyring, err := NewKeyring(options.SecretKeys, options.ActiveKeyVersion)
	if err != nil {
		return nil, err
	}
	issuer := strings.TrimSpace(options.Issuer)
	if issuer == "" {
		issuer = "MW Identity"
	}
	if options.RecoveryTTL <= 0 {
		options.RecoveryTTL = 10 * time.Minute
	}
	return &Service{
		store: store, keyring: keyring, issuer: issuer,
		recoveryTTL: options.RecoveryTTL, now: time.Now,
	}, nil
}

func (s *Service) TOTPStatus(ctx context.Context, userID string) (bool, error) {
	record, err := s.store.IdentityTOTPByUserID(ctx, userID)
	if err != nil {
		if errorsIsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return record.Enabled, nil
}

func (s *Service) SetupTOTP(ctx context.Context, user auth.User) (TOTPSetup, error) {
	if existing, err := s.store.IdentityTOTPByUserID(ctx, user.ID); err == nil && existing.Enabled {
		return TOTPSetup{}, ErrTOTPAlreadyEnabled
	} else if err != nil && !errorsIsNotFound(err) {
		return TOTPSetup{}, err
	}

	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return TOTPSetup{}, fmt.Errorf("generate TOTP secret: %w", err)
	}
	defer zero(raw)

	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	ciphertext, nonce, version, err := s.keyring.Encrypt(raw, []byte("mw-identity-totp:"+user.ID))
	if err != nil {
		return TOTPSetup{}, err
	}
	now := s.now().UTC()
	if err := s.store.UpsertIdentityTOTP(ctx, TOTPRecord{
		UserID: user.ID, SecretCiphertext: ciphertext, SecretNonce: nonce,
		SecretKeyVersion: version, Enabled: false, LastAcceptedStep: -1,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return TOTPSetup{}, err
	}

	label := s.issuer + ":" + user.Email
	values := url.Values{}
	values.Set("secret", encoded)
	values.Set("issuer", s.issuer)
	values.Set("algorithm", "SHA1")
	values.Set("digits", "6")
	values.Set("period", "30")
	uri := "otpauth://totp/" + url.PathEscape(label) + "?" + values.Encode()
	s.audit(ctx, user.ID, "identity.totp.setup.started", "success", "started")
	return TOTPSetup{Secret: encoded, OTPAuthURI: uri}, nil
}

func (s *Service) ConfirmTOTP(ctx context.Context, userID, code string) error {
	record, secret, err := s.loadTOTP(ctx, userID)
	if err != nil {
		return err
	}
	defer zero(secret)
	step, ok := ValidateTOTP(secret, code, s.now(), record.LastAcceptedStep)
	if !ok {
		s.audit(ctx, userID, "identity.totp.confirm.failed", "failure", "invalid_code")
		return ErrInvalidCode
	}
	if err := s.store.AcceptIdentityTOTPStep(ctx, userID, step, true, s.now().UTC()); err != nil {
		return err
	}
	s.audit(ctx, userID, "identity.totp.enabled", "success", "confirmed")
	return nil
}

func (s *Service) DisableTOTP(ctx context.Context, userID string) error {
	if err := s.store.DeleteIdentityTOTP(ctx, userID); err != nil {
		return err
	}
	s.audit(ctx, userID, "identity.totp.disabled", "success", "disabled")
	return nil
}

func (s *Service) AuthenticateTOTP(ctx context.Context, email, code string) (auth.User, error) {
	normalized, err := auth.NormalizeEmail(email)
	if err != nil {
		return auth.User{}, ErrInvalidCode
	}
	user, err := s.store.IdentityUserByEmail(ctx, normalized)
	if err != nil || !user.Enabled {
		return auth.User{}, ErrInvalidCode
	}
	pending, err := s.store.IdentityUserRecoveryPending(ctx, user.ID)
	if err != nil {
		return auth.User{}, err
	}
	if pending {
		return auth.User{}, auth.ErrRecoveryPending
	}

	record, secret, err := s.loadTOTP(ctx, user.ID)
	if err != nil || !record.Enabled {
		return auth.User{}, ErrInvalidCode
	}
	defer zero(secret)
	step, ok := ValidateTOTP(secret, code, s.now(), record.LastAcceptedStep)
	if !ok {
		s.audit(ctx, user.ID, "identity.totp.login.failed", "failure", "invalid_code")
		return auth.User{}, ErrInvalidCode
	}
	if err := s.store.AcceptIdentityTOTPStep(ctx, user.ID, step, false, s.now().UTC()); err != nil {
		return auth.User{}, ErrInvalidCode
	}
	s.audit(ctx, user.ID, "identity.totp.login.completed", "success", "authenticated")
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) StartRecovery(ctx context.Context, email string) (RecoveryStarted, error) {
	normalized, err := auth.NormalizeEmail(email)
	if err != nil {
		return RecoveryStarted{}, ErrRecovery
	}
	user, err := s.store.IdentityUserByEmail(ctx, normalized)
	if err != nil || !user.Enabled {
		return RecoveryStarted{}, ErrRecovery
	}
	token, err := auth.NewOpaqueToken("mwrec_")
	if err != nil {
		return RecoveryStarted{}, err
	}
	id, err := auth.NewOpaqueToken("rec_")
	if err != nil {
		return RecoveryStarted{}, err
	}
	now := s.now().UTC()
	request := RecoveryRequest{
		ID: id, UserID: user.ID, TokenHash: auth.HashToken(token),
		Status: "pending", ExpiresAt: now.Add(s.recoveryTTL), CreatedAt: now,
	}
	if err := s.store.BeginIdentityRecovery(ctx, request, now); err != nil {
		return RecoveryStarted{}, err
	}
	s.audit(ctx, user.ID, "identity.recovery.started", "success", "operator_started")
	user.PasswordHash = ""
	return RecoveryStarted{User: user, Token: token, ExpiresAt: request.ExpiresAt}, nil
}

func (s *Service) Recovery(ctx context.Context, rawToken string) (RecoveryRequest, auth.User, error) {
	if strings.TrimSpace(rawToken) == "" {
		return RecoveryRequest{}, auth.User{}, ErrRecovery
	}
	request, user, err := s.store.IdentityRecoveryByTokenHash(ctx, auth.HashToken(rawToken), s.now().UTC())
	if err != nil {
		return RecoveryRequest{}, auth.User{}, err
	}
	if request.Status != "pending" || !request.ExpiresAt.After(s.now().UTC()) {
		return RecoveryRequest{}, auth.User{}, ErrRecovery
	}
	user.PasswordHash = ""
	return request, user, nil
}

func (s *Service) CompleteRecovery(ctx context.Context, requestID, userID string) error {
	now := s.now().UTC()
	if err := s.store.ConsumeIdentityRecovery(ctx, requestID, userID, now); err != nil {
		return err
	}
	s.audit(ctx, userID, "identity.recovery.completed", "success", "passkey_replaced")
	return nil
}

func (s *Service) CancelRecovery(ctx context.Context, email string) error {
	normalized, err := auth.NormalizeEmail(email)
	if err != nil {
		return ErrRecovery
	}
	user, err := s.store.IdentityUserByEmail(ctx, normalized)
	if err != nil {
		return ErrRecovery
	}
	if err := s.store.CancelIdentityRecovery(ctx, user.ID, s.now().UTC()); err != nil {
		return err
	}
	s.audit(ctx, user.ID, "identity.recovery.cancelled", "success", "operator_cancelled")
	return nil
}

func (s *Service) RevokeUserSessions(ctx context.Context, actorUserID, targetUserID string) error {
	if _, err := s.store.IdentityUserByID(ctx, targetUserID); err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.store.RevokeIdentitySessionsAndTokens(ctx, targetUserID, now); err != nil {
		return err
	}
	s.audit(ctx, targetUserID, "identity.sessions.revoked", "success", "admin_revocation")
	_ = actorUserID
	return nil
}

func (s *Service) loadTOTP(ctx context.Context, userID string) (TOTPRecord, []byte, error) {
	record, err := s.store.IdentityTOTPByUserID(ctx, userID)
	if err != nil {
		return TOTPRecord{}, nil, err
	}
	secret, err := s.keyring.Decrypt(record.SecretCiphertext, record.SecretNonce, record.SecretKeyVersion, []byte("mw-identity-totp:"+userID))
	if err != nil {
		return TOTPRecord{}, nil, err
	}
	return record, secret, nil
}

func (s *Service) audit(ctx context.Context, userID, eventType, status, reason string) {
	id, err := auth.NewOpaqueToken("audit_")
	if err != nil {
		return
	}
	var user *string
	if userID != "" {
		value := userID
		user = &value
	}
	_ = s.store.RecordAuditEvent(ctx, audit.Event{
		ID: id, EventType: eventType, Status: status, Severity: "info",
		UserID: user, ReasonCode: reason, CreatedAt: s.now().UTC(),
	})
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, auth.ErrNotFound)
}
