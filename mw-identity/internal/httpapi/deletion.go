package httpapi

import (
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"html/template"
	"net/http"
	"time"
)

const deletionStatusCookie = "mw_deletion_status"

var deletionProgressPage = template.Must(template.New("deletion").Funcs(template.FuncMap{"deletionLabel": deletionLabel}).Parse(`<!doctype html><html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta http-equiv="refresh" content="10"><title>{{.Title}}</title><link rel="stylesheet" href="/static/portal.css"></head><body class="identity-page"><main class="account-shell narrow-shell"><section class="settings-card"><h1>{{.Title}}</h1><p>{{.Explanation}}</p><p>{{deletionLabel .Lang .Operation.Status}}</p><ul>{{range .Operation.Deliveries}}<li><strong>{{.Name}}</strong>: {{deletionLabel $.Lang .Status}}</li>{{end}}</ul><p>{{.Limit}}</p></section></main></body></html>`))

func (h *identityAdminHandlers) listDeletions(w http.ResponseWriter, r *http.Request) {
	_, pErr, err := h.principal(r, identityadmin.PermissionUsersRead)
	if h.handlePrincipalError(w, pErr, err) {
		return
	}
	store, err := h.auth.DurableDeletionStore()
	if err != nil {
		writeJSONError(w, 503, "deletion_unavailable")
		return
	}
	operations, err := store.ListDeletionOperations(r.Context())
	if err != nil {
		writeJSONError(w, 500, "deletion_status_failed")
		return
	}
	// Subjects are visible only to the authenticated operator, never public progress.
	rows := []map[string]any{}
	for _, op := range operations {
		rows = append(rows, map[string]any{"id": op.ID, "subject": op.Subject, "status": op.Status, "requested_at": op.RequestedAt})
	}
	writeJSON(w, 200, map[string]any{"operations": rows})
}
func (h *identityAdminHandlers) reconcileLegacyDeletion(w http.ResponseWriter, r *http.Request) {
	principal, pErr, err := h.principal(r, identityadmin.PermissionUsersWrite)
	if h.handlePrincipalError(w, pErr, err) {
		return
	}
	if !recentWebAuthnAccess(principal.access, h.strongMaxAge, time.Now()) {
		writeStepUpRequired(w)
		return
	}
	var request struct {
		Outcomes  map[string]string `json:"outcomes"`
		Reference string            `json:"evidence_reference"`
	}
	if !decodeJSONBody(w, r, &request) {
		return
	}
	store, err := h.auth.DurableDeletionStore()
	if err != nil {
		writeJSONError(w, 503, "deletion_unavailable")
		return
	}
	if err = store.ReconcileLegacyDeletion(r.Context(), r.PathValue("id"), request.Outcomes, request.Reference, time.Now().UTC()); err != nil {
		writeJSONError(w, 409, "legacy_reconciliation_incomplete")
		return
	}
	h.auth.AuditDeletionRetry(r.Context(), principal.access.User.ID, r.PathValue("id"), remoteIP(r), false)
	writeJSON(w, 200, map[string]string{"status": "reconciled"})
}
func (h *identityAuthHandlers) deletionMetrics(w http.ResponseWriter, r *http.Request) {
	store, err := h.service.DurableDeletionStore()
	if err != nil {
		writeJSONError(w, 503, "deletion_unavailable")
		return
	}
	stats, err := store.DeletionQueueStats(r.Context(), time.Now().UTC())
	if err != nil {
		writeJSONError(w, 503, "deletion_metrics_unavailable")
		return
	}
	writeJSON(w, 200, stats)
}

func (h *identityAuthHandlers) deletionProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	capability := cookieValue(r, deletionStatusCookie)
	store, err := h.service.DurableDeletionStore()
	if err != nil || capability == "" {
		writePlainError(w, 401, "Deletion status requires its original browser cookie; contact an authorized operator for recovery.")
		return
	}
	op, err := store.DeletionByCapability(r.Context(), auth.HashToken(capability), time.Now().UTC())
	if err != nil {
		writePlainError(w, 401, "Deletion status expired or unavailable; contact an authorized operator.")
		return
	}
	title, explanation, limit, lang := "Exclusão da identidade MW", "A identidade está desativada. A exclusão final depende dos resultados de cada aplicativo.", "Cópias externas e backups dos aplicativos não são confirmados por esta tela.", "pt-BR"
	if pageLanguage(r) == "en" {
		title, explanation, limit, lang = "MW identity deletion", "The identity is disabled. Final deletion depends on each application's required outcome.", "This screen does not confirm removal of external copies or application backups.", "en"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = deletionProgressPage.Execute(w, map[string]any{"Title": title, "Explanation": explanation, "Limit": limit, "Lang": lang, "Operation": op})
}
func (h *identityAdminHandlers) deletionStatus(w http.ResponseWriter, r *http.Request) {
	_, pErr, err := h.principal(r, identityadmin.PermissionUsersRead)
	if h.handlePrincipalError(w, pErr, err) {
		return
	}
	store, err := h.auth.DurableDeletionStore()
	if err != nil {
		writeJSONError(w, 503, "deletion_unavailable")
		return
	}
	op, err := store.DeletionOperation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSONError(w, 404, "not_found")
		return
	}
	writeJSON(w, 200, op)
}
func (h *identityAdminHandlers) retryDeletion(w http.ResponseWriter, r *http.Request) {
	principal, pErr, err := h.principal(r, identityadmin.PermissionUsersWrite)
	if h.handlePrincipalError(w, pErr, err) {
		return
	}
	if !recentWebAuthnAccess(principal.access, h.strongMaxAge, time.Now()) {
		writeStepUpRequired(w)
		return
	}
	var request struct {
		RefreshSnapshot bool `json:"refresh_snapshot"`
	}
	if !decodeJSONBody(w, r, &request) {
		return
	}
	store, err := h.auth.DurableDeletionStore()
	if err != nil {
		writeJSONError(w, 503, "deletion_unavailable")
		return
	}
	if err = store.RetryDeletionDelivery(r.Context(), r.PathValue("event"), request.RefreshSnapshot, time.Now().UTC()); err != nil {
		writeJSONError(w, 409, "deletion_not_retryable")
		return
	}
	h.auth.AuditDeletionRetry(r.Context(), principal.access.User.ID, r.PathValue("event"), remoteIP(r), request.RefreshSnapshot)
	writeJSON(w, 202, map[string]string{"status": "pending"})
}

func deletionLabel(lang, status string) string {
	en := map[string]string{"pending": "Pending application cleanup", "delivering": "Contacting application", "confirmed_cleanup": "Required cleanup confirmed", "no_current_mapping": "No current linked profile", "exempt": "Infrastructure: no application profile", "failed": "Needs operator recovery", "unresolved": "Needs operator review", "legacy_review_required": "Needs review of an earlier deletion request", "completed": "MW identity deletion completed"}
	pt := map[string]string{"pending": "Aguardando limpeza nos aplicativos", "delivering": "Contatando aplicativo", "confirmed_cleanup": "Limpeza exigida confirmada", "no_current_mapping": "Nenhum perfil vinculado atualmente", "exempt": "Infraestrutura: sem perfil de aplicativo", "failed": "Requer recuperacao pelo operador", "unresolved": "Requer revisao pelo operador", "legacy_review_required": "Requer revisao de uma solicitacao anterior", "completed": "Exclusao da identidade MW concluida"}
	labels := pt
	if lang == "en" {
		labels = en
	}
	if label, ok := labels[status]; ok {
		return label
	}
	return status
}
