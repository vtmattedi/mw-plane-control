package auth

import (
	"context"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
)

// Only whole-MW-account deletion emits a destructive lifecycle callback.
const DeletionReasonAccountDeleted = "account_deleted"

func (s *Service) ConnectedApplications(ctx context.Context, userID string) ([]ConnectedApplication, error) {
	return s.store.ListIdentityUserApplications(ctx, userID)
}

func (s *Service) DeleteAccount(ctx context.Context, userID, remoteIP string) (DeleteAccountResult, error) {
	user, err := s.store.IdentityUserByID(ctx, userID)
	if err != nil {
		return DeleteAccountResult{}, err
	}
	if user.IsSuperadmin {
		return DeleteAccountResult{}, ErrAccountDeletionForbidden
	}

	return s.beginDurableDeletion(ctx, userID)
}

// RemoveConnectedApplication revokes sign-in consent, preserving app data and assignments.
func (s *Service) RemoveConnectedApplication(ctx context.Context, userID, applicationID, remoteIP string) (DeletionDelivery, error) {
	apps, err := s.store.ListIdentityUserDeletionApplications(ctx, userID)
	if err != nil {
		return DeletionDelivery{}, err
	}
	for _, app := range apps {
		if app.ID != applicationID {
			continue
		}
		if err := s.store.RevokeIdentityUserApplicationAccess(ctx, userID, app.ClientID); err != nil {
			return DeletionDelivery{}, err
		}
		s.recordAudit(ctx, audit.Event{ID: mustAuditID(), EventType: "identity.application.disconnected", Status: "success", Severity: "info", UserID: &userID, RemoteIP: remoteIP, CreatedAt: s.now().UTC()})
		return DeletionDelivery{ApplicationID: app.ID, ClientID: app.ClientID, Status: "disconnected"}, nil
	}
	return DeletionDelivery{}, ErrNotFound
}
