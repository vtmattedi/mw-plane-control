package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
)

type identityAdminHandlers struct {
	oidc         *oidc.Service
	admin        *identityadmin.Service
	strong       *identitystrong.Service
	auth         *auth.Service
	strongMaxAge time.Duration
}

type identityAdminPrincipal struct {
	access      oidc.AccessContext
	role        string
	permissions []string
}

func newIdentityAdminHandlers(oidcService *oidc.Service, adminService *identityadmin.Service, strongService *identitystrong.Service, authService *auth.Service, strongMaxAge time.Duration) *identityAdminHandlers {
	return &identityAdminHandlers{oidc: oidcService, admin: adminService, strong: strongService, auth: authService, strongMaxAge: strongMaxAge}
}

func (h *identityAdminHandlers) deleteUser(w http.ResponseWriter, r *http.Request) {
	principal, protocolErr, err := h.principal(r, identityadmin.PermissionUsersWrite)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}
	target := strings.TrimSpace(r.PathValue("user_id"))
	if target == "" || target == principal.access.User.ID {
		writeJSONError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := h.auth.DeleteAccount(r.Context(), target, remoteIP(r))
	if errors.Is(err, auth.ErrAccountDeletionForbidden) {
		writeJSONError(w, http.StatusForbidden, "account_deletion_forbidden")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "identity_admin_failed")
		return
	}
	status := http.StatusOK
	if !result.Deleted {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result)
}

func (h *identityAdminHandlers) principal(
	r *http.Request,
	requiredPermission string,
) (
	identityAdminPrincipal,
	*oidc.ProtocolError,
	error,
) {
	access, protocolErr :=
		h.oidc.ValidateAccessToken(
			r.Context(),
			bearerToken(r),
			oidc.IdentityAdminAudience,
			"identity.admin",
		)
	if protocolErr != nil {
		return identityAdminPrincipal{},
			protocolErr,
			nil
	}

	permissions, role, err :=
		h.admin.Permissions(
			r.Context(),
			access.User,
			access.Application.ClientID,
		)
	if err != nil {
		if errors.Is(
			err,
			identityadmin.ErrRoleForbidden,
		) {
			return identityAdminPrincipal{},
				&oidc.ProtocolError{
					Code:        "insufficient_scope",
					Description: "Identity admin permission denied",
					HTTPStatus:  http.StatusForbidden,
				},
				nil
		}
		return identityAdminPrincipal{},
			nil,
			err
	}

	if requiredPermission != "" &&
		!identityadmin.HasPermission(
			permissions,
			requiredPermission,
		) {
		return identityAdminPrincipal{},
			&oidc.ProtocolError{
				Code:        "insufficient_scope",
				Description: "Identity admin permission denied",
				HTTPStatus:  http.StatusForbidden,
			},
			nil
	}

	if requiresRecentAdminWebAuthn(r) && !recentWebAuthnAccess(access, h.strongMaxAge, time.Now()) {
		return identityAdminPrincipal{}, &oidc.ProtocolError{Code: "step_up_required", Description: "recent WebAuthn required", HTTPStatus: http.StatusForbidden}, nil
	}

	return identityAdminPrincipal{
		access:      access,
		role:        role,
		permissions: permissions,
	}, nil, nil
}

func (h *identityAdminHandlers) whoami(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, protocolErr, err :=
		h.principal(r, "")
	if h.handlePrincipalError(
		w,
		protocolErr,
		err,
	) {
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"sub":         principal.access.User.ID,
			"email":       principal.access.User.Email,
			"role":        principal.role,
			"permissions": principal.permissions,
		},
	)
}

func (h *identityAdminHandlers) listApplications(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionApplicationsRead,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	apps, err := h.admin.ListApplications(
		r.Context(),
	)
	if err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"identity_admin_failed",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"applications": apps,
		},
	)
}

func (h *identityAdminHandlers) getApplication(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionApplicationsRead,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	app, err := h.admin.Application(
		r.Context(),
		r.PathValue("id"),
	)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, app)
}

func (h *identityAdminHandlers) createApplication(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionApplicationsWrite,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	var request struct {
		Name                   string   `json:"name"`
		ClientID               string   `json:"client_id"`
		ApplicationType        string   `json:"application_type"`
		AccessPolicy           string   `json:"access_policy"`
		RedirectURIs           []string `json:"redirect_uris"`
		PostLogoutRedirectURIs []string `json:"post_logout_redirect_uris"`
		DeleteCallbackURI      string   `json:"delete_callback_uri"`
		AccountManagementURI   string   `json:"account_management_uri"`
		AccountDeletionURI     string   `json:"account_deletion_uri"`
		Internal               bool     `json:"internal"`
		Scopes                 []string `json:"scopes"`
		Claims                 []string `json:"claims"`
		Audiences              []string `json:"audiences"`
	}

	if !decodeJSONBody(w, r, &request) {
		return
	}

	result, err := h.admin.CreateApplication(
		r.Context(),
		principal.access.User,
		remoteIP(r),
		identityadmin.CreateApplicationRequest{
			Name:                   request.Name,
			ClientID:               request.ClientID,
			ApplicationType:        request.ApplicationType,
			AccessPolicy:           request.AccessPolicy,
			RedirectURIs:           request.RedirectURIs,
			PostLogoutRedirectURIs: request.PostLogoutRedirectURIs,
			DeleteCallbackURI:      request.DeleteCallbackURI,
			AccountManagementURI:   request.AccountManagementURI,
			AccountDeletionURI:     request.AccountDeletionURI,
			Internal:               request.Internal,
			Scopes:                 request.Scopes,
			Claims:                 request.Claims,
			Audiences:              request.Audiences,
		},
	)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, applicationCreationResponse(result))
}

func applicationCreationResponse(result identityadmin.CreateApplicationResult) map[string]any {
	response := map[string]any{"application": result.Application}
	if result.ClientSecret != "" {
		response["client_secret"] = result.ClientSecret
	}
	return response
}

func (h *identityAdminHandlers) updateApplication(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionApplicationsWrite,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	var request identityadmin.UpdateApplication
	if !decodeJSONBody(w, r, &request) {
		return
	}

	app, err := h.admin.UpdateApplication(
		r.Context(),
		principal.access.User,
		remoteIP(r),
		r.PathValue("id"),
		request,
	)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, app)
}

func (h *identityAdminHandlers) applicationState(
	action string,
) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		principal, protocolErr, err := h.principal(
			r,
			identityadmin.PermissionApplicationsWrite,
		)
		if h.handlePrincipalError(
			w,
			protocolErr,
			err,
		) {
			return
		}

		app, err := h.admin.SetApplicationState(
			r.Context(),
			principal.access.User,
			remoteIP(r),
			r.PathValue("id"),
			action,
		)
		if err != nil {
			h.writeAdminError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, app)
	}
}

func (h *identityAdminHandlers) deleteApplication(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionApplicationsWrite,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	err = h.admin.DeleteApplication(
		r.Context(),
		principal.access.User,
		remoteIP(r),
		r.PathValue("id"),
	)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *identityAdminHandlers) rotateSecret(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionApplicationsRotate,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	result, err := h.admin.RotateSecret(
		r.Context(),
		principal.access.User,
		remoteIP(r),
		r.PathValue("id"),
	)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"client_secret": result.ClientSecret,
		},
	)
}

func (h *identityAdminHandlers) users(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionUsersRead,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	users, err := h.admin.Users(
		r.Context(),
	)
	if err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"identity_admin_failed",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"users": users,
		},
	)
}

func (h *identityAdminHandlers) createUser(w http.ResponseWriter, r *http.Request) {
	principal, protocolErr, err := h.principal(r, identityadmin.PermissionUsersWrite)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	var request identityadmin.CreateUserRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}
	user, err := h.admin.CreateUser(r.Context(), principal.access.User, remoteIP(r), request)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (h *identityAdminHandlers) assignments(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionAssignmentsRead,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	values, err := h.admin.Assignments(
		r.Context(),
		r.PathValue("id"),
	)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"assignments": values,
		},
	)
}

func (h *identityAdminHandlers) upsertAssignment(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionAssignmentsWrite,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	var request struct {
		Role         string `json:"role"`
		AccessEffect string `json:"access_effect"`
	}
	if !decodeJSONBody(w, r, &request) {
		return
	}

	value, err := h.admin.UpsertAssignment(
		r.Context(),
		principal.access.User,
		remoteIP(r),
		r.PathValue("id"),
		r.PathValue("user_id"),
		request.Role,
		request.AccessEffect,
	)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, value)
}

func (h *identityAdminHandlers) deleteAssignment(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionAssignmentsWrite,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	err = h.admin.DeleteAssignment(
		r.Context(),
		principal.access.User,
		remoteIP(r),
		r.PathValue("id"),
		r.PathValue("user_id"),
	)
	if err != nil {
		h.writeAdminError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *identityAdminHandlers) audit(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, protocolErr, err := h.principal(
		r,
		identityadmin.PermissionAuditRead,
	)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}

	limit := 100
	if raw := strings.TrimSpace(
		r.URL.Query().Get("limit"),
	); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil {
			limit = parsed
		}
	}

	entries, err := h.admin.Audit(
		r.Context(),
		limit,
	)
	if err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"identity_admin_failed",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"audit": entries,
		},
	)
}

func (h *identityAdminHandlers) handlePrincipalError(
	w http.ResponseWriter,
	protocolErr *oidc.ProtocolError,
	err error,
) bool {
	if err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"identity_admin_failed",
		)
		return true
	}
	if protocolErr != nil {
		if protocolErr.Code == "invalid_token" {
			w.Header().Set(
				"WWW-Authenticate",
				`Bearer error="invalid_token"`,
			)
		}
		writeOAuthError(w, protocolErr)
		return true
	}
	return false
}

func (h *identityAdminHandlers) writeAdminError(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		identityadmin.ErrNotFound,
	),
		errors.Is(
			err,
			applications.ErrApplicationNotFound,
		):
		writeJSONError(
			w,
			http.StatusNotFound,
			"not_found",
		)

	case errors.Is(
		err,
		identityadmin.ErrInvalid,
	),
		errors.Is(
			err,
			applications.ErrInvalidApplication,
		):
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid_request",
		)

	case errors.Is(err, applications.ErrDeleteCallbackRequired):
		writeJSONError(
			w,
			http.StatusBadRequest,
			"delete_callback_required",
		)

	case errors.Is(
		err,
		identityadmin.ErrArchived,
	):
		writeJSONError(
			w,
			http.StatusConflict,
			"application_archived",
		)

	case errors.Is(err, identityadmin.ErrArchiveRequired):
		writeJSONError(
			w,
			http.StatusConflict,
			"application_archive_required",
		)

	case errors.Is(err, identityadmin.ErrApplicationInUse):
		writeJSONError(
			w,
			http.StatusConflict,
			"application_in_use",
		)

	case errors.Is(
		err,
		applications.ErrClientIDExists,
	):
		writeJSONError(
			w,
			http.StatusConflict,
			"client_id_exists",
		)

	case errors.Is(err, auth.ErrEmailExists):
		writeJSONError(w, http.StatusConflict, "email_exists")

	case errors.Is(err, identityadmin.ErrRoleForbidden):
		writeJSONError(w, http.StatusForbidden, "admin_access_cannot_be_denied")

	default:
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"identity_admin_failed",
		)
	}
}

func decodeJSONBody(
	w http.ResponseWriter,
	r *http.Request,
	target any,
) bool {
	if !strings.HasPrefix(
		strings.ToLower(
			r.Header.Get("Content-Type"),
		),
		"application/json",
	) {
		writeJSONError(
			w,
			http.StatusUnsupportedMediaType,
			"json_required",
		)
		return false
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		256*1024,
	)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid_json",
		)
		return false
	}

	return true
}

func requiresRecentAdminWebAuthn(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (h *identityAdminHandlers) revokeUserSessions(w http.ResponseWriter, r *http.Request) {
	principal, protocolErr, err := h.principal(r, identityadmin.PermissionApplicationsWrite)
	if h.handlePrincipalError(w, protocolErr, err) {
		return
	}
	target := strings.TrimSpace(r.PathValue("user_id"))
	if target == "" {
		writeJSONError(w, 400, "invalid_request")
		return
	}
	if err := h.strong.RevokeUserSessions(r.Context(), principal.access.User.ID, target); err != nil {
		writeJSONError(w, 500, "identity_admin_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
