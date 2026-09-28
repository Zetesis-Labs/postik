package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/auth"
	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/connect"
	"github.com/zetesis-labs/postik/internal/core/access"
	"github.com/zetesis-labs/postik/internal/core/identity"
	"github.com/zetesis-labs/postik/internal/library"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/storage"
)

var Languages = []string{"es", "en"}

type IdentityReader interface {
	GetUser(ctx context.Context, id uuid.UUID) (*postgres.User, error)
	Memberships(ctx context.Context, userID uuid.UUID) ([]postgres.MembershipView, error)
}

type Server struct {
	Superadmin access.Superadmin
	OIDC       *config.OIDC
	Sessions   *auth.Sessions
	Identity   IdentityReader
	Channels   *postgres.Channels
	Telegram   *connect.Telegram
	Media      *library.Library
	Posts      *postgres.Posts
	Files      storage.Files
	Now        func() time.Time
	Logger     *slog.Logger
}

var _ StrictServerInterface = (*Server)(nil)

func errorBody(code, message string) ErrorJSONResponse {
	return ErrorJSONResponse{Code: code, Message: message}
}

var (
	errInvalidCredentials = errorBody("invalid_credentials", "Invalid credentials")
	errUnauthenticated    = errorBody("unauthenticated", "Sign in to continue")
	errNotFound           = errorBody("not_found", "Not found")
	errNotAMember         = errors.New("the session does not belong to a member")
)

func (s *Server) GetInstance(ctx context.Context, _ GetInstanceRequestObject) (GetInstanceResponseObject, error) {
	instance := GetInstance200JSONResponse{
		SuperadminTotp: s.Superadmin.HasTOTP(),
		Languages:      Languages,
	}
	if s.OIDC != nil {
		instance.Oidc = &OidcProvider{Name: s.OIDC.DisplayName}
	}
	return instance, nil
}

func (s *Server) LoginSuperadmin(ctx context.Context, request LoginSuperadminRequestObject) (LoginSuperadminResponseObject, error) {
	attempt := access.SuperadminAttempt{
		Username: request.Body.Username,
		Password: request.Body.Password,
	}
	if request.Body.Code != nil {
		attempt.Code = *request.Body.Code
	}
	if !access.CheckSuperadmin(s.Superadmin, attempt, s.Now()) {
		return LoginSuperadmin401JSONResponse{errInvalidCredentials}, nil
	}
	cookie, err := s.Sessions.Open(ctx, access.SessionSuperadmin, nil)
	if err != nil {
		return nil, err
	}
	return LoginSuperadmin200JSONResponse{
		Body:    Me{Kind: Superadmin},
		Headers: LoginSuperadmin200ResponseHeaders{SetCookie: &cookie},
	}, nil
}

func (s *Server) Logout(ctx context.Context, _ LogoutRequestObject) (LogoutResponseObject, error) {
	cookie, err := s.Sessions.Close(ctx)
	if err != nil {
		return nil, err
	}
	return Logout204Response{Headers: Logout204ResponseHeaders{SetCookie: &cookie}}, nil
}

// member is the person behind a member session, their organizations and the
// one they are working in.
type member struct {
	user        *postgres.User
	memberships []postgres.MembershipView
	active      uuid.UUID
}

func (s *Server) loadMember(ctx context.Context, principal auth.Principal) (member, error) {
	if principal.Kind != access.SessionMember || principal.UserID == nil {
		return member{}, errNotAMember
	}
	user, err := s.Identity.GetUser(ctx, *principal.UserID)
	if err != nil {
		return member{}, err
	}
	memberships, err := s.Identity.Memberships(ctx, user.ID)
	if err != nil {
		return member{}, err
	}
	active, _ := identity.ActiveOrganization(toCore(memberships), principal.RememberedOrganization)
	return member{user: user, memberships: memberships, active: active}, nil
}

func toCore(views []postgres.MembershipView) []identity.Membership {
	memberships := make([]identity.Membership, len(views))
	for i, v := range views {
		memberships[i] = identity.Membership{OrganizationID: v.OrganizationID, JoinedAt: v.JoinedAt}
	}
	return memberships
}

func (s *Server) GetMe(ctx context.Context, _ GetMeRequestObject) (GetMeResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return GetMe401JSONResponse{errUnauthenticated}, nil
	}
	if principal.Kind == access.SessionSuperadmin {
		return GetMe200JSONResponse{Kind: Superadmin}, nil
	}
	m, err := s.loadMember(ctx, principal)
	if err != nil {
		return nil, err
	}
	organizations := make([]MeOrganization, len(m.memberships))
	for i, v := range m.memberships {
		organizations[i] = MeOrganization{Id: v.OrganizationID, Name: v.OrganizationName, Role: MeOrganizationRole(v.Role)}
	}
	me := GetMe200JSONResponse{
		Kind:          Member,
		User:          &MeUser{Id: m.user.ID, Name: m.user.Name, Email: m.user.Email},
		Organizations: &organizations,
	}
	if m.active != uuid.Nil {
		me.ActiveOrganizationId = &m.active
	}
	return me, nil
}

func (s *Server) SetActiveOrganization(ctx context.Context, request SetActiveOrganizationRequestObject) (SetActiveOrganizationResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok || principal.Kind != access.SessionMember {
		return SetActiveOrganization401JSONResponse{errUnauthenticated}, nil
	}
	m, err := s.loadMember(ctx, principal)
	if err != nil {
		return nil, err
	}
	requested := request.Body.OrganizationId
	for _, v := range m.memberships {
		if v.OrganizationID == requested {
			cookie := s.Sessions.OrganizationCookie(requested)
			return SetActiveOrganization204Response{Headers: SetActiveOrganization204ResponseHeaders{SetCookie: &cookie}}, nil
		}
	}
	return SetActiveOrganization404JSONResponse(errNotFound), nil
}
