package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/delegation"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

type DelegationProbeConfig struct {
	AppEnv      string
	ResourceURL string
	Audience    string
	Scope       string
	HTTPTimeout time.Duration
}

type delegationProbe struct {
	delegation *delegation.Client
	sessions   *session.Service
	config     DelegationProbeConfig
	httpClient *http.Client
}

func newDelegationProbe(
	client *delegation.Client,
	sessions *session.Service,
	config DelegationProbeConfig,
) *delegationProbe {
	if client == nil || sessions == nil ||
		strings.EqualFold(strings.TrimSpace(config.AppEnv), "production") ||
		strings.TrimSpace(config.ResourceURL) == "" ||
		strings.TrimSpace(config.Audience) == "" ||
		strings.TrimSpace(config.Scope) == "" {
		return nil
	}
	return &delegationProbe{
		delegation: client,
		sessions:   sessions,
		config:     config,
		httpClient: &http.Client{Timeout: config.HTTPTimeout},
	}
}

func (p *delegationProbe) serve(
	auth *authHandlers,
	w http.ResponseWriter,
	r *http.Request,
) {
	noStore(w)
	_, record, err := auth.current(r)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	subjectToken, err := p.sessions.IdentityAccessToken(record)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "identity_session_unavailable")
		return
	}
	defer zeroStringBestEffort(&subjectToken)

	token, err := p.delegation.Token(
		r.Context(),
		record.ID,
		subjectToken,
		p.config.Audience,
		[]string{p.config.Scope},
	)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "delegation_failed")
		return
	}
	defer zeroStringBestEffort(&token.AccessToken)

	request, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		strings.TrimRight(p.config.ResourceURL, "/")+"/protected",
		nil,
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "resource_request_failed")
		return
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)

	response, err := p.httpClient.Do(request)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "resource_unavailable")
		return
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "resource_response_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if token.Cached {
		w.Header().Set("X-MW-Delegation-Cache", "hit")
	} else {
		w.Header().Set("X-MW-Delegation-Cache", "miss")
	}
	w.WriteHeader(response.StatusCode)

	if json.Valid(body) {
		_, _ = w.Write(body)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "resource_invalid_json",
	})
}
