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
	if s.OAuth != nil {
		for _, identifier := range s.OAuth.Providers() {
			providers = append(providers, Provider{Identifier: identifier, Name: providerNames[identifier]})
		}
	}
	if s.Telegram != nil {
		providers = append(providers, Provider{Identifier: "telegram", Name: "Telegram"})
	}
	return providers, nil
}

// providerNames are the names Postiz shows in the grid of «Añadir canal».
var providerNames = map[string]string{
	"linkedin":      "LinkedIn",
	"linkedin-page": "LinkedIn Page",
	"x":             "X",
	"telegram":      "Telegram",
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

var (
	errNetworkUnreachable  = errorBody("network_unreachable", "The network could not be reached")
	errNotInBetweenSteps   = errorBody("not_in_between_steps", "The channel is not waiting for a page")
	errPageNotAdministered = errorBody("page_not_administered", "You do not administer this page")
)

// activeMember returns the member of the session and the organization they
// work in, or false when there is none.
func (s *Server) activeMember(ctx context.Context) (uuid.UUID, uuid.UUID, bool, error) {
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

func (s *Server) StartChannelAuthorization(ctx context.Context, request StartChannelAuthorizationRequestObject) (StartChannelAuthorizationResponseObject, error) {
	user, org, ok, err := s.activeMember(ctx)
	if err != nil || !ok {
		return StartChannelAuthorization401JSONResponse{errUnauthenticated}, err
	}
	if s.OAuth == nil {
		return StartChannelAuthorization404JSONResponse(errProviderUnavailable), nil
	}
	url, err := s.OAuth.Start(ctx, org, user, request.Provider, request.Body.ChannelId)
	switch {
	case errors.Is(err, connect.ErrUnknownProvider):
		return StartChannelAuthorization404JSONResponse(errProviderUnavailable), nil
	case errors.Is(err, connect.ErrChannelNotFound):
		return StartChannelAuthorization404JSONResponse(errChannelNotFound), nil
	case err != nil:
		s.Logger.ErrorContext(ctx, "start the authorization", "provider", request.Provider, "error", err)
		return StartChannelAuthorization502JSONResponse(errNetworkUnreachable), nil
	}
	return StartChannelAuthorization201JSONResponse{Url: url}, nil
}

func (s *Server) ListChannelPages(ctx context.Context, request ListChannelPagesRequestObject) (ListChannelPagesResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return ListChannelPages401JSONResponse{errUnauthenticated}, err
	}
	if s.OAuth == nil {
		return ListChannelPages404JSONResponse(errChannelNotFound), nil
	}
	pages, err := s.OAuth.Pages(ctx, org, request.Id)
	switch {
	case errors.Is(err, connect.ErrChannelNotFound):
		return ListChannelPages404JSONResponse(errChannelNotFound), nil
	case errors.Is(err, connect.ErrNotInBetweenSteps):
		return ListChannelPages409JSONResponse(errNotInBetweenSteps), nil
	case err != nil:
		s.Logger.ErrorContext(ctx, "list the pages", "channel", request.Id, "error", err)
		return ListChannelPages502JSONResponse(errNetworkUnreachable), nil
	}
	out := make(ListChannelPages200JSONResponse, len(pages))
	for i, p := range pages {
		out[i] = ChannelPage{Id: p.ID, Name: p.Name}
		if p.PictureURL != "" {
			out[i].Picture = &p.PictureURL
		}
	}
	return out, nil
}

func (s *Server) ChooseChannelPage(ctx context.Context, request ChooseChannelPageRequestObject) (ChooseChannelPageResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return ChooseChannelPage401JSONResponse{errUnauthenticated}, err
	}
	if s.OAuth == nil {
		return ChooseChannelPage404JSONResponse(errChannelNotFound), nil
	}
	channel, err := s.OAuth.ChoosePage(ctx, org, request.Id, request.Body.PageId)
	switch {
	case errors.Is(err, connect.ErrChannelNotFound):
		return ChooseChannelPage404JSONResponse(errChannelNotFound), nil
	case errors.Is(err, connect.ErrNotInBetweenSteps):
		return ChooseChannelPage409JSONResponse(errNotInBetweenSteps), nil
	case errors.Is(err, connect.ErrPageNotAdministered):
		return ChooseChannelPage403JSONResponse(errPageNotAdministered), nil
	case err != nil:
		s.Logger.ErrorContext(ctx, "choose the page", "channel", request.Id, "error", err)
		return ChooseChannelPage502JSONResponse(errNetworkUnreachable), nil
	}
	return ChooseChannelPage200JSONResponse(toChannel(*channel)), nil
}
