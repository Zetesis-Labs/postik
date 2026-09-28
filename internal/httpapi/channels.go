package httpapi

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/auth"
	"github.com/zetesis-labs/postik/internal/connect"
	"github.com/zetesis-labs/postik/internal/core/channels"
	"github.com/zetesis-labs/postik/internal/postgres"
)

var (
	errChannelNotFound     = errorBody("not_found", "Channel not found")
	errProviderUnavailable = errorBody("provider_not_available", "This provider is not configured")
	errTelegramUnreachable = errorBody("telegram_unreachable", "Telegram could not be reached")
	errBadPostingTimes     = errorBody("invalid_posting_times", "Posting times must be between 0 and 1439 minutes")
)

// activeOrganization returns the organization the member works in, or false
// when the request has no member session with an organization.
func (s *Server) activeOrganization(ctx context.Context) (uuid.UUID, bool, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return uuid.Nil, false, nil
	}
	m, err := s.loadMember(ctx, principal)
	if errors.Is(err, errNotAMember) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return m.active, m.active != uuid.Nil, nil
}

func (s *Server) ListProviders(ctx context.Context, _ ListProvidersRequestObject) (ListProvidersResponseObject, error) {
	if _, ok, err := s.activeOrganization(ctx); err != nil || !ok {
		return ListProviders401JSONResponse{errUnauthenticated}, err
	}
	providers := ListProviders200JSONResponse{}
	if s.Telegram != nil {
		providers = append(providers, Provider{Identifier: "telegram", Name: "Telegram"})
	}
	return providers, nil
}

func toChannel(c postgres.Channel) Channel {
	channel := Channel{
		Id:             c.ID,
		Provider:       c.Provider,
		Name:           c.Name,
		Username:       c.Username,
		Picture:        c.Picture,
		Disabled:       c.Disabled,
		RefreshNeeded:  c.RefreshNeeded,
		InBetweenSteps: c.InBetweenSteps,
		PostingTimes:   c.PostingTimes,
	}
	if channel.PostingTimes == nil {
		channel.PostingTimes = []int{}
	}
	if c.Customer != nil {
		channel.Customer = &Customer{Id: c.Customer.ID, Name: c.Customer.Name}
	}
	return channel
}

func (s *Server) ListChannels(ctx context.Context, _ ListChannelsRequestObject) (ListChannelsResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return ListChannels401JSONResponse{errUnauthenticated}, err
	}
	list, err := s.Channels.List(ctx, org)
	if err != nil {
		return nil, err
	}
	out := make(ListChannels200JSONResponse, len(list))
	for i, c := range list {
		out[i] = toChannel(c)
	}
	return out, nil
}

func (s *Server) DeleteChannel(ctx context.Context, request DeleteChannelRequestObject) (DeleteChannelResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return DeleteChannel401JSONResponse{errUnauthenticated}, err
	}
	channel, err := s.Channels.Get(ctx, org, request.Id)
	if errors.Is(err, postgres.ErrNotFound) {
		return DeleteChannel404JSONResponse(errChannelNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.Channels.Delete(ctx, org, request.Id); err != nil {
		return nil, err
	}
	if channel.Picture != nil {
		_ = s.Files.Remove(*channel.Picture)
	}
	return DeleteChannel204Response{}, nil
}

func (s *Server) SetChannelDisabled(ctx context.Context, request SetChannelDisabledRequestObject) (SetChannelDisabledResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return SetChannelDisabled401JSONResponse{errUnauthenticated}, err
	}
	err = s.Channels.SetDisabled(ctx, org, request.Id, request.Body.Disabled, s.Now())
	if errors.Is(err, postgres.ErrNotFound) {
		return SetChannelDisabled404JSONResponse(errChannelNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return SetChannelDisabled204Response{}, nil
}

func (s *Server) SetChannelPostingTimes(ctx context.Context, request SetChannelPostingTimesRequestObject) (SetChannelPostingTimesResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return SetChannelPostingTimes401JSONResponse(errUnauthenticated), err
	}
	times, err := channels.NormalizePostingTimes(request.Body.Times)
	if err != nil {
		return SetChannelPostingTimes400JSONResponse{errBadPostingTimes}, nil
	}
	err = s.Channels.SetPostingTimes(ctx, org, request.Id, times, s.Now())
	if errors.Is(err, postgres.ErrNotFound) {
		return SetChannelPostingTimes404JSONResponse(errChannelNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return SetChannelPostingTimes204Response{}, nil
}

func (s *Server) SetChannelCustomer(ctx context.Context, request SetChannelCustomerRequestObject) (SetChannelCustomerResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return SetChannelCustomer401JSONResponse{errUnauthenticated}, err
	}
	if exists, err := s.Channels.ExistsInOrganization(ctx, org, request.Id); err != nil || !exists {
		return SetChannelCustomer404JSONResponse(errChannelNotFound), err
	}
	customerID, found, err := s.resolveCustomer(ctx, org, request.Body)
	if err != nil {
		return nil, err
	}
	if !found {
		return SetChannelCustomer404JSONResponse(errorBody("not_found", "Customer not found")), nil
	}
	if err := s.Channels.SetCustomer(ctx, org, request.Id, customerID, s.Now()); err != nil {
		return nil, err
	}
	return SetChannelCustomer204Response{}, nil
}

// resolveCustomer turns the request into the customer to assign: by name
// (created if missing), by ID (must be of the organization) or none.
func (s *Server) resolveCustomer(ctx context.Context, org uuid.UUID, body *SetChannelCustomerJSONRequestBody) (*uuid.UUID, bool, error) {
	switch {
	case body.Name != nil:
		name := channels.CustomerName(*body.Name)
		if name == "" {
			return nil, true, nil
		}
		customer, err := s.Channels.CustomerByName(ctx, org, name, s.Now())
		if err != nil {
			return nil, false, err
		}
		return &customer.ID, true, nil
	case body.CustomerId != nil:
		customer, err := s.Channels.CustomerByID(ctx, org, *body.CustomerId)
		if errors.Is(err, postgres.ErrNotFound) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		return &customer.ID, true, nil
	default:
		return nil, true, nil
	}
}

func (s *Server) ListCustomers(ctx context.Context, _ ListCustomersRequestObject) (ListCustomersResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return ListCustomers401JSONResponse{errUnauthenticated}, err
	}
	list, err := s.Channels.Customers(ctx, org)
	if err != nil {
		return nil, err
	}
	out := make(ListCustomers200JSONResponse, len(list))
	for i, c := range list {
		out[i] = Customer{Id: c.ID, Name: c.Name}
	}
	return out, nil
}

func (s *Server) OpenTelegramConnection(ctx context.Context, _ OpenTelegramConnectionRequestObject) (OpenTelegramConnectionResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return OpenTelegramConnection401JSONResponse{errUnauthenticated}, err
	}
	if s.Telegram == nil {
		return OpenTelegramConnection404JSONResponse(errProviderUnavailable), nil
	}
	code, bot, err := s.Telegram.Open(ctx, org)
	if err != nil {
		s.Logger.ErrorContext(ctx, "open telegram connection", "error", err)
		return OpenTelegramConnection502JSONResponse(errTelegramUnreachable), nil
	}
	return OpenTelegramConnection201JSONResponse{Code: code, BotUsername: bot}, nil
}

func (s *Server) GetTelegramConnection(ctx context.Context, request GetTelegramConnectionRequestObject) (GetTelegramConnectionResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return GetTelegramConnection401JSONResponse{errUnauthenticated}, err
	}
	if s.Telegram == nil {
		return GetTelegramConnection404JSONResponse(errProviderUnavailable), nil
	}
	status, channelID, err := s.Telegram.Status(ctx, org, request.Code)
	if errors.Is(err, connect.ErrConnectionNotFound) {
		return GetTelegramConnection404JSONResponse(errorBody("not_found", "Connection not found")), nil
	}
	if err != nil {
		s.Logger.ErrorContext(ctx, "sync telegram connection", "error", err)
		return GetTelegramConnection502JSONResponse(errTelegramUnreachable), nil
	}
	out := GetTelegramConnection200JSONResponse{Status: TelegramConnectionStatusStatus(status)}
	if status == channels.Connected {
		out.ChannelId = channelID
	}
	return out, nil
}
