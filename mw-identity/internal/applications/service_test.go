package applications

import (
	"context"
	"testing"
)

type memoryApplicationStore struct {
	app Application
}

func (s *memoryApplicationStore) CreateApplication(
	_ context.Context,
	input CreateInput,
) error {
	s.app = Application{
		ID:                     input.ID,
		Name:                   input.Name,
		ClientID:               input.ClientID,
		ClientSecretHash:       input.ClientSecretHash,
		ApplicationType:        input.ApplicationType,
		AuthenticationPolicy:   input.AuthenticationPolicy,
		AccessPolicy:           input.AccessPolicy,
		Enabled:                input.Enabled,
		RedirectURIs:           input.RedirectURIs,
		PostLogoutRedirectURIs: input.PostLogoutRedirectURIs,
		Scopes:                 input.Scopes,
		Claims:                 input.Claims,
		Audiences:              input.Audiences,
		CreatedAt:              input.CreatedAt,
		UpdatedAt:              input.CreatedAt,
	}
	return nil
}

func (s *memoryApplicationStore) ApplicationByClientID(
	_ context.Context,
	clientID string,
) (Application, error) {
	if s.app.ClientID != clientID {
		return Application{}, ErrApplicationNotFound
	}
	return s.app, nil
}

func TestCreateConfidentialApplication(
	t *testing.T,
) {
	store := &memoryApplicationStore{}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Create(
		context.Background(),
		CreateRequest{
			Name:            "MW Admin",
			ClientID:        "mw_admin",
			ApplicationType: "confidential",
			RedirectURIs: []string{
				"https://admin.mattediworks.com/api/auth/callback",
			},
			Scopes: []string{
				"openid",
				"profile",
				"email",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.ClientSecret == "" {
		t.Fatal("confidential client secret missing")
	}
	if result.Application.AccessPolicy != "default_deny" {
		t.Fatalf("access policy = %q; want default_deny", result.Application.AccessPolicy)
	}
	if result.Application.ClientSecretHash ==
		result.ClientSecret {
		t.Fatal("raw client secret was retained as hash")
	}
	if !ClientSecretMatches(
		result.ClientSecret,
		result.Application.ClientSecretHash,
	) {
		t.Fatal("client secret hash does not verify")
	}
}

func TestRedirectMatchingIsExact(t *testing.T) {
	store := &memoryApplicationStore{}
	service, _ := NewService(store)

	_, err := service.Create(
		context.Background(),
		CreateRequest{
			Name:     "MW Admin",
			ClientID: "mw_admin",
			RedirectURIs: []string{
				"https://admin.mattediworks.com/api/auth/callback",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := service.RedirectAllowed(
		context.Background(),
		"mw_admin",
		"https://admin.mattediworks.com/api/auth/callback",
	)
	if err != nil || !ok {
		t.Fatalf(
			"exact redirect: ok=%v err=%v",
			ok,
			err,
		)
	}

	ok, err = service.RedirectAllowed(
		context.Background(),
		"mw_admin",
		"https://admin.mattediworks.com/api/auth/callback/extra",
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("non-exact redirect URI was allowed")
	}
}

func TestRedirectRequiresHTTPSOutsideLocalhost(
	t *testing.T,
) {
	store := &memoryApplicationStore{}
	service, _ := NewService(store)

	_, err := service.Create(
		context.Background(),
		CreateRequest{
			Name:     "Bad",
			ClientID: "bad",
			RedirectURIs: []string{
				"http://example.com/callback",
			},
		},
	)
	if err != ErrInvalidApplication {
		t.Fatalf(
			"error = %v; want ErrInvalidApplication",
			err,
		)
	}
}

func TestCreateTrimsInvisibleCharactersAroundURIs(t *testing.T) {
	store := &memoryApplicationStore{}
	service, _ := NewService(store)

	result, err := service.Create(context.Background(), CreateRequest{
		Name:            "Working On IT",
		ClientID:        "mw-woi",
		ApplicationType: "public",
		RedirectURIs: []string{
			"\u200bhttps://woi.mattediworks.com/api/auth/mw/callback\ufeff",
		},
		PostLogoutRedirectURIs: []string{
			"\u200bhttps://woi.mattediworks.com/",
		},
		DeleteCallbackURI: "\ufeffhttps://woi.mattediworks.com/api/auth/mw/lifecycle",
		Scopes:            []string{"openid", "profile", "email"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Application.RedirectURIs[0]; got != "https://woi.mattediworks.com/api/auth/mw/callback" {
		t.Fatalf("redirect URI = %q", got)
	}
	if got := result.Application.PostLogoutRedirectURIs[0]; got != "https://woi.mattediworks.com/" {
		t.Fatalf("post-logout URI = %q", got)
	}
	if got := result.Application.DeleteCallbackURI; got != "https://woi.mattediworks.com/api/auth/mw/lifecycle" {
		t.Fatalf("delete callback URI = %q", got)
	}
}
