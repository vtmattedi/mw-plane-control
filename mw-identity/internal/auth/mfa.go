package auth

import (
	"context"
	"fmt"
	"time"
)

type MFAChallenge struct {
	TokenHash, BindingHash, UserID, PasswordFingerprint, ReturnTo string
	ExpiresAt                                                     time.Time
}
type MFAStore interface {
	CreateMFAChallenge(context.Context, MFAChallenge) error
	AttemptMFAChallenge(context.Context, string, string, time.Time) (MFAChallenge, error)
	CompleteMFAChallenge(context.Context, string, string, time.Time) error
}

func (s *Service) BeginMFA(ctx context.Context, email string, password []byte, binding, returnTo, remoteIP string) (string, error) {
	user, err := s.VerifyPasswordForLogin(ctx, email, password, remoteIP)
	if err != nil {
		return "", err
	}
	store, ok := s.store.(MFAStore)
	if !ok {
		return "", fmt.Errorf("MFA persistence unavailable")
	}
	raw, err := NewOpaqueToken("mfa_")
	if err != nil {
		return "", err
	}
	err = store.CreateMFAChallenge(ctx, MFAChallenge{TokenHash: HashToken(raw), BindingHash: HashToken(binding), UserID: user.ID, PasswordFingerprint: HashToken(user.PasswordHash), ReturnTo: returnTo, ExpiresAt: s.now().UTC().Add(5 * time.Minute)})
	return raw, err
}
func (s *Service) AttemptMFA(ctx context.Context, raw, binding string) (MFAChallenge, User, error) {
	store, ok := s.store.(MFAStore)
	if !ok {
		return MFAChallenge{}, User{}, ErrInvalidLogin
	}
	challenge, err := store.AttemptMFAChallenge(ctx, HashToken(raw), HashToken(binding), s.now().UTC())
	if err != nil {
		return MFAChallenge{}, User{}, ErrInvalidLogin
	}
	user, err := s.store.IdentityUserByID(ctx, challenge.UserID)
	if err != nil || !user.Enabled || user.DeletionRequestedAt != nil || !TokenMatchesHash(user.PasswordHash, challenge.PasswordFingerprint) {
		return MFAChallenge{}, User{}, ErrInvalidLogin
	}
	pending, err := s.store.IdentityUserRecoveryPending(ctx, user.ID)
	if err != nil || pending {
		return MFAChallenge{}, User{}, ErrInvalidLogin
	}
	return challenge, user, nil
}
func (s *Service) FinishMFA(ctx context.Context, challenge MFAChallenge, binding, remoteIP string) (LoginResult, error) {
	store, ok := s.store.(MFAStore)
	if !ok {
		return LoginResult{}, ErrInvalidLogin
	}
	user, err := s.store.IdentityUserByID(ctx, challenge.UserID)
	if err != nil || !TokenMatchesHash(user.PasswordHash, challenge.PasswordFingerprint) {
		return LoginResult{}, ErrInvalidLogin
	}
	if err = store.CompleteMFAChallenge(ctx, challenge.TokenHash, HashToken(binding), s.now().UTC()); err != nil {
		return LoginResult{}, ErrInvalidLogin
	}
	return s.CreateSessionForUser(ctx, challenge.UserID, remoteIP, "password_totp")
}
