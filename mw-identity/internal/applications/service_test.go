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
			Internal:        true,
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
			Internal: true,
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

func TestCreateEnforcesTheDeletionContract(t *testing.T) {
	base := func() CreateRequest {
		return CreateRequest{
			Name:         "Your App",
			ClientID:     "your-app",
			RedirectURIs: []string{"https://app.example.com/callback"},
		}
	}

	cases := []struct {
		name   string
		modify func(*CreateRequest)
		want   error
	}{
		{"user-facing app without callback", func(*CreateRequest) {}, ErrDeleteCallbackRequired},
		{"user-facing app with blank callback", func(r *CreateRequest) { r.DeleteCallbackURI = "  " }, ErrDeleteCallbackRequired},
		{"user-facing app with callback", func(r *CreateRequest) { r.DeleteCallbackURI = "https://app.example.com/account-deleted" }, nil},
		{"internal app without callback", func(r *CreateRequest) { r.Internal = true }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := NewService(&memoryApplicationStore{})
			request := base()
			tc.modify(&request)
			result, err := service.Create(context.Background(), request)
			if err != tc.want {
				t.Fatalf("error = %v; want %v", err, tc.want)
			}
			if err == nil && result.Application.Internal != request.Internal {
				t.Fatalf("internal = %v; want %v", result.Application.Internal, request.Internal)
			}
		})
	}
}

func TestProductionCreateAllowsLocalhostOIDCRedirects(t *testing.T) {
	cases := []struct {
		name           string
		redirect       string
		postLogout     string
		deleteCallback string
		wantError      bool
	}{
		{
			name:           "localhost HTTP browser callbacks are valid with HTTPS lifecycle endpoint",
			redirect:       "http://localhost:3001/auth/callback",
			postLogout:     "http://localhost:5173/",
			deleteCallback: "https://calc.example.com/identity/account-deleted",
		},
		{
			name:           "external HTTP browser redirects remain invalid",
			redirect:       "http://example.com/auth/callback",
			deleteCallback: "https://calc.example.com/identity/account-deleted",
			wantError:      true,
		},
		{
			name:           "production localhost deletion callback remains invalid",
			redirect:       "http://localhost:3001/auth/callback",
			deleteCallback: "http://localhost:3001/auth/account-deleted",
			wantError:      true,
		},
		{
			name:           "production external HTTP deletion callback remains invalid",
			redirect:       "http://localhost:3001/auth/callback",
			deleteCallback: "http://calc.example.com/identity/account-deleted",
			wantError:      true,
		},
		{
			name:           "post-logout HTTP must be localhost",
			redirect:       "http://localhost:3001/auth/callback",
			postLogout:     "http://example.com/",
			deleteCallback: "https://calc.example.com/identity/account-deleted",
			wantError:      true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, err := NewService(&memoryApplicationStore{})
			if err != nil {
				t.Fatal(err)
			}
			service.RequireHTTPS(true)
			result, err := service.Create(context.Background(), CreateRequest{
				Name:                   "MW Calc",
				ClientID:               "mw-calc-local",
				ApplicationType:        "confidential",
				AccessPolicy:           "default_deny",
				RedirectURIs:           []string{tc.redirect},
				PostLogoutRedirectURIs: []string{tc.postLogout},
				DeleteCallbackURI:      tc.deleteCallback,
				Scopes:                 []string{"openid", "profile", "email"},
			})
			if tc.wantError {
				if err != ErrInvalidApplication {
					t.Fatalf("error = %v, want ErrInvalidApplication", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Application.RedirectURIs) != 1 ||
				result.Application.RedirectURIs[0] != tc.redirect {
				t.Fatalf("unexpected redirect URIs: %v", result.Application.RedirectURIs)
			}
		})
	}
}
