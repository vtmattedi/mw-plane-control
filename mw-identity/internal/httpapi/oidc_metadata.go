package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
)

type OIDCMetadataConfig struct {
	PublicIssuer   string
	SigningManager *signing.Manager
}

func discoveryHandler(
	publicIssuer string,
) http.HandlerFunc {
	document := oidc.Discovery(publicIssuer)

	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.Header().Set(
			"Content-Type",
			"application/json",
		)
		w.Header().Set(
			"Cache-Control",
			"public, max-age=300",
		)

		_ = json.NewEncoder(w).Encode(document)
	}
}

func jwksHandler(
	manager *signing.Manager,
) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		jwks, err := manager.PublicJWKS(
			r.Context(),
		)
		if err != nil {
			writePlainError(
				w,
				http.StatusInternalServerError,
				"could not load signing keys",
			)
			return
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)
		w.Header().Set(
			"Cache-Control",
			"public, max-age=300",
		)

		_ = json.NewEncoder(w).Encode(jwks)
	}
}
