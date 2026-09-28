package jobs

import (
	"context"

	"github.com/google/uuid"
)

// Notification is what the organization is told about a publication.
type Notification struct {
	Kind     string
	Template string
	Params   map[string]string
}

// Notifier tells an organization about its publications (§8).
type Notifier interface {
	Notify(ctx context.Context, organizationID uuid.UUID, n Notification) error
}

type nopNotifier struct{}

func (nopNotifier) Notify(context.Context, uuid.UUID, Notification) error { return nil }
