package httpapi

import (
	"context"
	"time"

	"github.com/zetesis-labs/postik/internal/auth"
	"github.com/zetesis-labs/postik/internal/core/access"
)

var Languages = []string{"es", "en"}

type Server struct {
	Superadmin access.Superadmin
	Sessions   *auth.Sessions
	Now        func() time.Time
}

var _ StrictServerInterface = (*Server)(nil)

func errorBody(code, message string) ErrorJSONResponse {
	return ErrorJSONResponse{Code: code, Message: message}
}

var (
	errInvalidCredentials = errorBody("invalid_credentials", "Invalid credentials")
	errUnauthenticated    = errorBody("unauthenticated", "Sign in to continue")
)

func (s *Server) GetInstance(ctx context.Context, _ GetInstanceRequestObject) (GetInstanceResponseObject, error) {
	return GetInstance200JSONResponse{
		SuperadminTotp: s.Superadmin.HasTOTP(),
		Languages:      Languages,
	}, nil
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

func (s *Server) GetMe(ctx context.Context, _ GetMeRequestObject) (GetMeResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return GetMe401JSONResponse{errUnauthenticated}, nil
	}
	return GetMe200JSONResponse{Kind: MeKind(principal.Kind)}, nil
}
