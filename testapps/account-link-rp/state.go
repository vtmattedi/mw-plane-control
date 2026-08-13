package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

type State struct {
	mu             sync.Mutex
	now            func() time.Time
	usersByID      map[string]LocalUser
	credentials    map[string]LocalCredential
	userIDByEmail  map[string]string
	sessions       map[string]Session
	transactions   map[string]OIDCTransaction
	pending        map[string]PendingLink
	identities     map[string]ExternalIdentity
	identityByUser map[string]string
	audit          []AuditEvent
	sessionTTL     time.Duration
	transactionTTL time.Duration
	pendingTTL     time.Duration
}

func NewState(cfg Config) *State {
	s := &State{
		now:            time.Now,
		usersByID:      map[string]LocalUser{},
		credentials:    map[string]LocalCredential{},
		userIDByEmail:  map[string]string{},
		sessions:       map[string]Session{},
		transactions:   map[string]OIDCTransaction{},
		pending:        map[string]PendingLink{},
		identities:     map[string]ExternalIdentity{},
		identityByUser: map[string]string{},
		audit:          []AuditEvent{},
		sessionTTL:     cfg.SessionTTL,
		transactionTTL: cfg.TransactionTTL,
		pendingTTL:     cfg.PendingTTL,
	}
	for _, credential := range cfg.Users {
		user := LocalUser{ID: credential.ID, Email: strings.TrimSpace(credential.Email)}
		s.usersByID[user.ID] = user
		s.credentials[user.ID] = credential
		s.userIDByEmail[normalizeEmail(user.Email)] = user.ID
	}
	return s
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hashOpaque(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomOpaque(prefix string, size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func passwordMatches(actual, expected string) bool {
	left := sha256.Sum256([]byte(actual))
	right := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(left[:], right[:]) == 1
}

func (s *State) AuthenticateLocal(email, password string) (LocalUser, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.userIDByEmail[normalizeEmail(email)]
	if !ok {
		return LocalUser{}, false
	}
	credential := s.credentials[userID]
	if !passwordMatches(password, credential.Password) {
		return LocalUser{}, false
	}
	return s.usersByID[userID], true
}

func (s *State) AuthenticateLocalUser(userID, password string) (LocalUser, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	credential, ok := s.credentials[userID]
	if !ok || !passwordMatches(password, credential.Password) {
		return LocalUser{}, false
	}
	return s.usersByID[userID], true
}

func (s *State) CreateSession(userID, authMethod string) (string, error) {
	raw, err := randomOpaque("lrps_", 32)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.usersByID[userID]; !ok {
		return "", ErrNotFound
	}
	now := s.now().UTC()
	s.sessions[hashOpaque(raw)] = Session{
		TokenHash:  hashOpaque(raw),
		UserID:     userID,
		AuthMethod: authMethod,
		ExpiresAt:  now.Add(s.sessionTTL),
	}
	return raw, nil
}

func (s *State) Session(raw string) (Session, LocalUser, error) {
	if strings.TrimSpace(raw) == "" {
		return Session{}, LocalUser{}, ErrInvalidSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := hashOpaque(raw)
	record, ok := s.sessions[hash]
	if !ok {
		return Session{}, LocalUser{}, ErrInvalidSession
	}
	if !record.ExpiresAt.After(s.now().UTC()) {
		delete(s.sessions, hash)
		return Session{}, LocalUser{}, ErrInvalidSession
	}
	user, ok := s.usersByID[record.UserID]
	if !ok {
		return Session{}, LocalUser{}, ErrInvalidSession
	}
	return record, user, nil
}

func (s *State) DeleteSession(raw string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, hashOpaque(raw))
}

func (s *State) DeleteSessionsForUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, session := range s.sessions {
		if session.UserID == userID {
			delete(s.sessions, key)
		}
	}
}

func (s *State) BeginTransaction(mode, localUserID, sessionHash string) (OIDCTransaction, string, string, error) {
	state, err := randomOpaque("", 32)
	if err != nil {
		return OIDCTransaction{}, "", "", err
	}
	nonce, err := randomOpaque("", 32)
	if err != nil {
		return OIDCTransaction{}, "", "", err
	}
	verifier, err := randomOpaque("", 64)
	if err != nil {
		return OIDCTransaction{}, "", "", err
	}
	record := OIDCTransaction{
		Mode:        mode,
		StateHash:   hashOpaque(state),
		NonceHash:   hashOpaque(nonce),
		Verifier:    verifier,
		LocalUserID: localUserID,
		SessionHash: sessionHash,
		ExpiresAt:   s.now().UTC().Add(s.transactionTTL),
	}
	s.mu.Lock()
	s.transactions[record.StateHash] = record
	s.mu.Unlock()
	return record, state, nonce, nil
}

func (s *State) ClaimTransaction(rawState string) (OIDCTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashOpaque(rawState)
	record, ok := s.transactions[key]
	if !ok {
		return OIDCTransaction{}, ErrNotFound
	}
	delete(s.transactions, key)
	if !record.ExpiresAt.After(s.now().UTC()) {
		return OIDCTransaction{}, ErrExpired
	}
	return record, nil
}

func identityKey(issuer, subject string) string {
	return issuer + "\x00" + subject
}

func userIssuerKey(userID, issuer string) string {
	return userID + "\x00" + issuer
}

func (s *State) LinkIdentity(userID string, identity IdentityClaims) (ExternalIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.linkIdentityLocked(userID, identity)
}

func (s *State) linkIdentityLocked(userID string, identity IdentityClaims) (ExternalIdentity, error) {
	if _, ok := s.usersByID[userID]; !ok {
		return ExternalIdentity{}, ErrNotFound
	}
	key := identityKey(identity.Issuer, identity.Subject)
	if existing, ok := s.identities[key]; ok {
		if existing.LocalUserID == userID {
			return existing, nil
		}
		return ExternalIdentity{}, ErrDuplicateSubject
	}
	userKey := userIssuerKey(userID, identity.Issuer)
	if existingKey, ok := s.identityByUser[userKey]; ok && existingKey != key {
		return ExternalIdentity{}, ErrIssuerAlreadyLink
	}
	now := s.now().UTC()
	record := ExternalIdentity{
		LocalUserID: userID,
		Issuer:      identity.Issuer,
		Subject:     identity.Subject,
		LinkedAt:    now,
	}
	s.identities[key] = record
	s.identityByUser[userKey] = key
	return record, nil
}

func (s *State) Identity(issuer, subject string) (ExternalIdentity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.identities[identityKey(issuer, subject)]
	return record, ok
}

func (s *State) MarkIdentityLogin(issuer, subject string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := identityKey(issuer, subject)
	record, ok := s.identities[key]
	if !ok {
		return
	}
	record.LastLoginAt = s.now().UTC()
	s.identities[key] = record
}

func (s *State) IdentitiesForUser(userID string) []ExternalIdentity {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := []ExternalIdentity{}
	for _, identity := range s.identities {
		if identity.LocalUserID == userID {
			values = append(values, identity)
		}
	}
	return values
}

func (s *State) UnlinkIdentity(userID, issuer string) (ExternalIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userKey := userIssuerKey(userID, issuer)
	key, ok := s.identityByUser[userKey]
	if !ok {
		return ExternalIdentity{}, ErrNotFound
	}
	record := s.identities[key]
	delete(s.identities, key)
	delete(s.identityByUser, userKey)
	return record, nil
}

func (s *State) CreatePending(identity IdentityClaims) (string, error) {
	raw, err := randomOpaque("lrpp_", 32)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[hashOpaque(raw)] = PendingLink{
		TokenHash: hashOpaque(raw),
		Identity:  identity,
		ExpiresAt: s.now().UTC().Add(s.pendingTTL),
	}
	return raw, nil
}

func (s *State) Pending(raw string) (PendingLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashOpaque(raw)
	record, ok := s.pending[key]
	if !ok {
		return PendingLink{}, ErrNotFound
	}
	if !record.ExpiresAt.After(s.now().UTC()) {
		delete(s.pending, key)
		return PendingLink{}, ErrExpired
	}
	return record, nil
}

func (s *State) CompletePending(raw, userID string) (ExternalIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashOpaque(raw)
	pending, ok := s.pending[key]
	if !ok {
		return ExternalIdentity{}, ErrNotFound
	}
	if !pending.ExpiresAt.After(s.now().UTC()) {
		delete(s.pending, key)
		return ExternalIdentity{}, ErrExpired
	}
	linked, err := s.linkIdentityLocked(userID, pending.Identity)
	if err != nil {
		return ExternalIdentity{}, err
	}
	delete(s.pending, key)
	return linked, nil
}

func (s *State) Audit(event AuditEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = s.now().UTC()
	}
	s.audit = append(s.audit, event)
}

func (s *State) AuditEvents() []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AuditEvent, len(s.audit))
	copy(out, s.audit)
	return out
}

func (s *State) AssertLinkSession(raw string, transaction OIDCTransaction) (LocalUser, error) {
	session, user, err := s.Session(raw)
	if err != nil {
		return LocalUser{}, err
	}
	if session.UserID != transaction.LocalUserID ||
		subtle.ConstantTimeCompare([]byte(session.TokenHash), []byte(transaction.SessionHash)) != 1 {
		return LocalUser{}, fmt.Errorf("link callback local session changed")
	}
	return user, nil
}
