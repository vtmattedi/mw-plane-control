package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/delegation"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/mwmqtt"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

const maxMQTTAdminRequestBytes int64 = 32 * 1024

type mqttAdminProxy struct {
	delegation   *delegation.Client
	sessions     *session.Service
	mqtt         *mwmqtt.Client
	audience     string
	strongMaxAge time.Duration
}

func newMQTTAdminProxy(
	delegationClient *delegation.Client,
	sessions *session.Service,
	mqttClient *mwmqtt.Client,
	audience string,
	strongMaxAge time.Duration,
) *mqttAdminProxy {
	audience = strings.TrimSpace(audience)
	if delegationClient == nil || sessions == nil ||
		mqttClient == nil || audience == "" {
		return nil
	}
	return &mqttAdminProxy{
		delegation:   delegationClient,
		sessions:     sessions,
		mqtt:         mqttClient,
		audience:     audience,
		strongMaxAge: strongMaxAge,
	}
}

func (p *mqttAdminProxy) serve(
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

	if requiresCSRF(r.Method) && !validAdminCSRF(r, record) {
		writeJSONError(w, http.StatusForbidden, "invalid_csrf")
		return
	}

	route, err := mwmqtt.Resolve(r.Method, "/"+r.PathValue("path"))
	if errors.Is(err, mwmqtt.ErrRouteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_mqtt_route")
		return
	}
	if sensitiveMQTTRoute(r.Method, route.DownstreamPath) && !requireBFFWebAuthn(w, record, p.strongMaxAge) {
		return
	}

	body, err := readMQTTAdminBody(w, r)
	if err != nil {
		if errors.Is(err, errMQTTRequestTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request_too_large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_request_body")
		return
	}

	subjectToken, err := p.sessions.IdentityAccessToken(record)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "identity_session_unavailable")
		return
	}
	defer zeroStringBestEffort(&subjectToken)

	delegated, err := p.delegation.Token(
		r.Context(),
		record.ID,
		subjectToken,
		p.audience,
		[]string{route.Scope},
	)
	if err != nil {
		// The error carries only Identity's HTTP status and OAuth error code, never a
		// token or secret. Without this line a refused delegation is indistinguishable
		// from a missing audience, a missing scope or a stale login.
		slog.WarnContext(
			r.Context(),
			"MQTT admin delegation failed",
			"audience", p.audience,
			"scope", route.Scope,
			"error", err,
		)
		writeJSONError(w, http.StatusBadGateway, "mqtt_delegation_failed")
		return
	}
	defer zeroStringBestEffort(&delegated.AccessToken)

	response, err := p.mqtt.Do(
		r.Context(),
		r.Method,
		route.DownstreamPath,
		r.URL.RawQuery,
		delegated.AccessToken,
		r.Header.Get("Content-Type"),
		r.Header.Get("Accept"),
		r.Header.Get("X-Request-ID"),
		body,
	)
	if err != nil {
		slog.WarnContext(
			r.Context(),
			"MQTT admin upstream request failed",
			"path", route.DownstreamPath,
			"error", err,
		)
		writeJSONError(w, http.StatusBadGateway, "mqtt_upstream_unavailable")
		return
	}

	if delegated.Cached {
		w.Header().Set("X-MW-Delegation-Cache", "hit")
	} else {
		w.Header().Set("X-MW-Delegation-Cache", "miss")
	}
	if response.RequestID != "" {
		w.Header().Set("X-Request-ID", response.RequestID)
	}

	// A downstream 401 means MQTT admin rejected the control-plane federation
	// credentials/token. The browser still has a valid BFF session.
	if response.StatusCode == http.StatusUnauthorized {
		writeJSONError(w, http.StatusBadGateway, "mqtt_federation_rejected")
		return
	}
	// The broker being unreachable is an actionable, non-sensitive condition: pass the
	// admin API's own `broker_unavailable` through so the UI can say so. Every other 5xx
	// stays a generic gateway error.
	if response.StatusCode == http.StatusServiceUnavailable {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"broker_unavailable"}`))
		return
	}
	if response.StatusCode >= 500 {
		writeJSONError(w, http.StatusBadGateway, "mqtt_upstream_failed")
		return
	}

	if response.ContentType != "" {
		w.Header().Set("Content-Type", response.ContentType)
	}
	w.WriteHeader(response.StatusCode)
	if len(response.Body) > 0 {
		_, _ = w.Write(response.Body)
	}
}

var errMQTTRequestTooLarge = errors.New("MQTT admin request too large")

func readMQTTAdminBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	limited := http.MaxBytesReader(w, r.Body, maxMQTTAdminRequestBytes)
	defer limited.Close()
	body, err := io.ReadAll(limited)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, errMQTTRequestTooLarge
		}
		return nil, err
	}
	return body, nil
}
