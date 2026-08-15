package applications

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

type Service struct {
	store Store
	now   func() time.Time
}

type CreateRequest struct {
	Name                   string
	ClientID               string
	ApplicationType        string
	AuthenticationPolicy   string
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	DeleteCallbackURI      string
	AllowPublicSignup      bool
	Scopes                 []string
	Claims                 []string
	Audiences              []string
}

type CreateResult struct {
	Application  Application
	ClientSecret string
}

func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf(
			"application store is required",
		)
	}

	return &Service{
		store: store,
		now:   time.Now,
	}, nil
}

func (s *Service) Create(
	ctx context.Context,
	request CreateRequest,
) (CreateResult, error) {
	name := strings.TrimSpace(request.Name)
	clientID := strings.TrimSpace(request.ClientID)
	appType := strings.TrimSpace(request.ApplicationType)
	policy := strings.TrimSpace(
		request.AuthenticationPolicy,
	)

	if name == "" || clientID == "" {
		return CreateResult{}, ErrInvalidApplication
	}

	if appType == "" {
		appType = "confidential"
	}
	if appType != "confidential" &&
		appType != "public" {
		return CreateResult{}, ErrInvalidApplication
	}

	if policy == "" {
		policy = "standard"
	}
	if policy != "standard" && policy != "strong" {
		return CreateResult{}, ErrInvalidApplication
	}

	redirectURIs, err := normalizeURIs(
		request.RedirectURIs,
		true,
	)
	if err != nil || len(redirectURIs) == 0 {
		return CreateResult{}, ErrInvalidApplication
	}

	postLogout, err := normalizeURIs(
		request.PostLogoutRedirectURIs,
		false,
	)
	if err != nil {
		return CreateResult{}, ErrInvalidApplication
	}
	deleteCallback, err := normalizeOptionalURI(request.DeleteCallbackURI, true)
	if err != nil {
		return CreateResult{}, ErrInvalidApplication
	}

	scopes := normalizeValues(request.Scopes)
	if len(scopes) == 0 {
		scopes = []string{
			"email",
			"openid",
			"profile",
		}
	}
	if !contains(scopes, "openid") {
		return CreateResult{}, ErrInvalidApplication
	}

	claims := normalizeValues(request.Claims)
	audiences := normalizeValues(request.Audiences)

	appID, err := randomID("app_")
	if err != nil {
		return CreateResult{}, err
	}

	var rawSecret string
	var secretHash string

	if appType == "confidential" {
		rawSecret, err = randomID("mw_client_")
		if err != nil {
			return CreateResult{}, err
		}
		secretHash = HashClientSecret(rawSecret)
	}

	now := s.now().UTC()

	input := CreateInput{
		ID:                     appID,
		Name:                   name,
		ClientID:               clientID,
		ClientSecretHash:       secretHash,
		ApplicationType:        appType,
		AuthenticationPolicy:   policy,
		Enabled:                true,
		RedirectURIs:           redirectURIs,
		PostLogoutRedirectURIs: postLogout,
		DeleteCallbackURI:      deleteCallback,
		AllowPublicSignup:      request.AllowPublicSignup,
		Scopes:                 scopes,
		Claims:                 claims,
		Audiences:              audiences,
		CreatedAt:              now,
	}

	if err := s.store.CreateApplication(
		ctx,
		input,
	); err != nil {
		return CreateResult{}, err
	}

	app := Application{
		ID:                     input.ID,
		Name:                   input.Name,
		ClientID:               input.ClientID,
		ClientSecretHash:       input.ClientSecretHash,
		ApplicationType:        input.ApplicationType,
		AuthenticationPolicy:   input.AuthenticationPolicy,
		Enabled:                input.Enabled,
		RedirectURIs:           append([]string(nil), input.RedirectURIs...),
		PostLogoutRedirectURIs: append([]string(nil), input.PostLogoutRedirectURIs...),
		DeleteCallbackURI:      input.DeleteCallbackURI,
		AllowPublicSignup:      input.AllowPublicSignup,
		Scopes:                 append([]string(nil), input.Scopes...),
		Claims:                 append([]string(nil), input.Claims...),
		Audiences:              append([]string(nil), input.Audiences...),
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	return CreateResult{
		Application:  app,
		ClientSecret: rawSecret,
	}, nil
}

func normalizeOptionalURI(value string, requiredHTTPS bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	values, err := normalizeURIs([]string{value}, requiredHTTPS)
	if err != nil || len(values) != 1 {
		return "", fmt.Errorf("invalid URI")
	}
	return values[0], nil
}

func (s *Service) RedirectAllowed(
	ctx context.Context,
	clientID string,
	redirectURI string,
) (bool, error) {
	app, err := s.store.ApplicationByClientID(
		ctx,
		strings.TrimSpace(clientID),
	)
	if err != nil {
		return false, err
	}
	if !app.Enabled || app.ArchivedAt != nil {
		return false, nil
	}

	for _, allowed := range app.RedirectURIs {
		if allowed == redirectURI {
			return true, nil
		}
	}

	return false, nil
}

func HashClientSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func ClientSecretMatches(
	raw string,
	expectedHash string,
) bool {
	actual := HashClientSecret(raw)
	if len(actual) != len(expectedHash) {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(actual),
		[]byte(expectedHash),
	) == 1
}

func normalizeURIs(
	values []string,
	requiredHTTPS bool,
) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})

	for _, value := range values {
		value = trimPastedValue(value)
		if value == "" {
			continue
		}

		parsed, err := url.Parse(value)
		if err != nil ||
			parsed.Scheme == "" ||
			parsed.Host == "" ||
			parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid URI")
		}

		if requiredHTTPS &&
			parsed.Scheme != "https" &&
			parsed.Hostname() != "localhost" {
			return nil, fmt.Errorf(
				"redirect URI must use https outside localhost",
			)
		}

		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	sort.Strings(result)
	return result, nil
}

func trimPastedValue(value string) string {
	return strings.TrimFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\u200b' || r == '\ufeff'
	})
}

func normalizeValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	sort.Strings(result)
	return result
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func randomID(prefix string) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf(
			"generate application identifier: %w",
			err,
		)
	}

	return prefix +
		base64.RawURLEncoding.EncodeToString(raw), nil
}
