package audit

import (
	"context"
	"encoding/json"
	"time"
)

type Event struct {
	ID         string
	EventType  string
	Status     string
	Severity   string
	UserID     *string
	RemoteIP   string
	ReasonCode string
	Metadata   map[string]any
	CreatedAt  time.Time
}

type Recorder interface {
	RecordAuditEvent(context.Context, Event) error
}

func MetadataJSON(metadata map[string]any) string {
	if len(metadata) == 0 {
		return "{}"
	}

	body, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}

	return string(body)
}
