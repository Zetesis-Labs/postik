package jobs

import (
	"context"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/core/notifications"
)

// Notification is what the organization is told about a publication.
type Notification = notifications.Notice

// Notifier tells an organization about its publications (§8).
type Notifier interface {
	Notify(ctx context.Context, organizationID uuid.UUID, n Notification) error
}

// Digester sends the hourly summary of the successful publications.
type Digester interface {
	SendDigests(ctx context.Context) error
}

type nopNotifier struct{}

func (nopNotifier) Notify(context.Context, uuid.UUID, Notification) error { return nil }
