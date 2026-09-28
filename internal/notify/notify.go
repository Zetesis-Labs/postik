// Package notify records what happens to an organization's publications and
// emails it to the members who want it (§8).
package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/core/notifications"
	"github.com/zetesis-labs/postik/internal/postgres"
)

// Sender sends one email.
type Sender interface {
	Send(ctx context.Context, to []string, subject, html string) error
}

type Service struct {
	Store     *postgres.Notifications
	Email     Sender
	PublicURL string
	Now       func() time.Time
	Logger    *slog.Logger
}

// Notify stores the notification and emails the informational ones to every
// member and the failures to those who want them. Successes wait for the digest.
func (s *Service) Notify(ctx context.Context, orgID uuid.UUID, n notifications.Notice) error {
	row := &postgres.Notification{
		ID: uuid.New(), OrganizationID: orgID, Kind: n.Kind, Template: n.Template, Params: n.Params, CreatedAt: s.Now(),
	}
	if err := s.Store.Insert(ctx, row); err != nil {
		return err
	}
	if s.Email == nil || n.Kind == "success" {
		return nil
	}
	recipients, err := s.Store.Recipients(ctx, orgID)
	if err != nil {
		return err
	}
	subject, html := notifications.Email(n, s.PublicURL)
	for _, r := range recipients {
		if n.Kind == "failure" && !r.EmailFailure {
			continue
		}
		s.send(ctx, r.Email, subject, html)
	}
	return nil
}

// SendDigests emails, per organization, the successes not yet summarized to
// the members who want them, and marks them as summarized.
func (s *Service) SendDigests(ctx context.Context) error {
	pending, err := s.Store.Undigested(ctx)
	if err != nil {
		return err
	}
	byOrganization := map[uuid.UUID][]postgres.Notification{}
	var order []uuid.UUID
	for _, n := range pending {
		if _, seen := byOrganization[n.OrganizationID]; !seen {
			order = append(order, n.OrganizationID)
		}
		byOrganization[n.OrganizationID] = append(byOrganization[n.OrganizationID], n)
	}
	for _, orgID := range order {
		items := byOrganization[orgID]
		if s.Email != nil {
			if err := s.digest(ctx, orgID, items); err != nil {
				return err
			}
		}
		ids := make([]uuid.UUID, len(items))
		for i, n := range items {
			ids[i] = n.ID
		}
		if err := s.Store.MarkDigested(ctx, ids, s.Now()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) digest(ctx context.Context, orgID uuid.UUID, items []postgres.Notification) error {
	recipients, err := s.Store.Recipients(ctx, orgID)
	if err != nil {
		return err
	}
	notices := make([]notifications.Notice, len(items))
	for i, n := range items {
		notices[i] = notifications.Notice{Kind: n.Kind, Template: n.Template, Params: n.Params}
	}
	subject, html := notifications.Digest(notices, s.PublicURL)
	for _, r := range recipients {
		if r.EmailSuccess {
			s.send(ctx, r.Email, subject, html)
		}
	}
	return nil
}

// send logs instead of failing: a lost email must not undo what it reports.
func (s *Service) send(ctx context.Context, to, subject, html string) {
	if err := s.Email.Send(ctx, []string{to}, subject, html); err != nil {
		s.Logger.ErrorContext(ctx, "send email", "to", to, "subject", subject, "error", err)
	}
}
