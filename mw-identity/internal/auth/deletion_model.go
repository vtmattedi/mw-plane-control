package auth

import (
	"context"
	"time"
)

type DeletionOperation struct {
	StatusCredentialIssued bool              `json:"-"`
	ID                     string            `json:"id"`
	Subject                string            `json:"-"`
	RequestedAt            time.Time         `json:"requested_at"`
	Status                 string            `json:"status"`
	Deliveries             []DurableDelivery `json:"deliveries"`
}

// Infrastructure has no local profile; an ordinary missing callback is never
// an acknowledgement of cleanup in either delivery mode.
func InitialDeletionStatus(internal bool, callback string) string {
	if internal {
		return "exempt"
	}
	if callback == "" {
		return "unresolved"
	}
	return "pending"
}

func DeletionOutcomeIsTerminal(status string) bool {
	return status == "confirmed_cleanup" || status == "no_current_mapping" || status == "exempt"
}

type DurableDelivery struct {
	RetryStartedAt time.Time `json:"-"`
	EventID        string    `json:"event_id"`
	OperationID    string    `json:"operation_id"`
	ApplicationID  string    `json:"application_id"`
	ClientID       string    `json:"client_id"`
	Name           string    `json:"name"`
	CallbackURI    string    `json:"-"`
	Status         string    `json:"status"`
	Attempts       int       `json:"attempts"`
	NextAttempt    time.Time `json:"next_attempt"`
	Lease          string    `json:"-"`
	ErrorCode      string    `json:"error_code,omitempty"`
	Subject        string    `json:"-"`
	RequestedAt    time.Time `json:"-"`
}
type DurableDeletionStore interface {
	ListDeletionOperations(context.Context) ([]DeletionOperation, error)
	DeletionQueueStats(context.Context, time.Time) (map[string]int64, error)
	ReconcileLegacyDeletion(context.Context, string, map[string]string, string, time.Time) error
	BeginDurableDeletion(context.Context, string, time.Time, string, time.Time) (DeletionOperation, error)
	DeletionOperation(context.Context, string) (DeletionOperation, error)
	DeletionByCapability(context.Context, string, time.Time) (DeletionOperation, error)
	ClaimDeletionDelivery(context.Context, time.Time, string) (DurableDelivery, error)
	FinishDeletionDelivery(context.Context, DurableDelivery, string, string, time.Time) error
	FinalizeDurableDeletion(context.Context, string, time.Time) error
	FinalizeReadyDeletions(context.Context, time.Time) error
	RetryDeletionDelivery(context.Context, string, bool, time.Time) error
}
