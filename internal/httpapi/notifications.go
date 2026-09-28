package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/auth"
	"github.com/zetesis-labs/postik/internal/postgres"
)

const notificationPageSize = 10

// memberAndOrganization is the signed-in member and their active organization.
func (s *Server) memberAndOrganization(ctx context.Context) (uuid.UUID, uuid.UUID, bool, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return uuid.Nil, uuid.Nil, false, nil
	}
	m, err := s.loadMember(ctx, principal)
	if errors.Is(err, errNotAMember) {
		return uuid.Nil, uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, false, err
	}
	return m.user.ID, m.active, m.active != uuid.Nil, nil
}

func (s *Server) ListNotifications(ctx context.Context, _ ListNotificationsRequestObject) (ListNotificationsResponseObject, error) {
	userID, org, ok, err := s.memberAndOrganization(ctx)
	if err != nil || !ok {
		return ListNotifications401JSONResponse{errUnauthenticated}, err
	}
	readAt, err := s.Notifications.ReadAt(ctx, userID)
	if err != nil {
		return nil, err
	}
	latest, err := s.Notifications.Latest(ctx, org, notificationPageSize)
	if err != nil {
		return nil, err
	}
	unread, err := s.Notifications.Unread(ctx, org, readAt)
	if err != nil {
		return nil, err
	}
	out := ListNotifications200JSONResponse{Items: make([]Notification, len(latest)), Unread: unread}
	for i, n := range latest {
		out.Items[i] = toNotification(n, readAt)
	}
	return out, nil
}

func toNotification(n postgres.Notification, readAt *time.Time) Notification {
	params := n.Params
	if params == nil {
		params = map[string]string{}
	}
	return Notification{
		Id: n.ID, Kind: NotificationKind(n.Kind), Template: n.Template, Params: params,
		CreatedAt: n.CreatedAt.UTC(), Unread: readAt == nil || n.CreatedAt.After(*readAt),
	}
}

func (s *Server) ReadNotifications(ctx context.Context, _ ReadNotificationsRequestObject) (ReadNotificationsResponseObject, error) {
	userID, _, ok, err := s.memberAndOrganization(ctx)
	if err != nil || !ok {
		return ReadNotifications401JSONResponse{errUnauthenticated}, err
	}
	if err := s.Notifications.MarkRead(ctx, userID, s.Now()); err != nil {
		return nil, err
	}
	return ReadNotifications204Response{}, nil
}

func (s *Server) SetEmailPreferences(ctx context.Context, request SetEmailPreferencesRequestObject) (SetEmailPreferencesResponseObject, error) {
	userID, _, ok, err := s.memberAndOrganization(ctx)
	if err != nil || !ok {
		return SetEmailPreferences401JSONResponse{errUnauthenticated}, err
	}
	preferences := postgres.Preferences{EmailSuccess: request.Body.EmailSuccess, EmailFailure: request.Body.EmailFailure}
	if err := s.Notifications.SetPreferences(ctx, userID, preferences); err != nil {
		return nil, err
	}
	return SetEmailPreferences204Response{}, nil
}
