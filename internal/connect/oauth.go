package connect

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/core/channels"
	"github.com/zetesis-labs/postik/internal/core/oauth"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/secretbox"
	"github.com/zetesis-labs/postik/internal/storage"
	"github.com/zetesis-labs/postik/internal/tokens"
)

var (
	ErrUnknownProvider     = errors.New("the provider is not configured")
	ErrChannelNotFound     = errors.New("channel not found")
	ErrNotInBetweenSteps   = errors.New("the channel is not waiting for a page")
	ErrPageNotAdministered = errors.New("the member does not administer the page")
	ErrMissingPermissions  = errors.New("the network did not grant every scope")
)

// Begin is where the browser goes to authorize, and how the authorization
// is known when it comes back.
type Begin struct {
	URL    string
	State  string
	Secret []byte
}

// Account is the network account an authorization gave access to.
type Account struct {
	ExternalID string
	Name       string
	Username   string
	PictureURL string
	Tokens     tokens.Tokens
	// Pending means a page is still to be chosen (F5, step 5).
	Pending bool
}

// Network is a network that connects with OAuth.
type Network interface {
	Begin(ctx context.Context, redirect string) (Begin, error)
	// State reads the key of the authorization from the callback query.
	State(q url.Values) string
	Denied(q url.Values) bool
	// Finish exchanges the grant and reads the account. It wraps
	// ErrMissingPermissions when a scope is missing.
	Finish(ctx context.Context, q url.Values, secret []byte, redirect string) (Account, error)
	Picture(ctx context.Context, url string) ([]byte, error)
}

// Page is a page a member can publish as.
type Page struct {
	ID         string
	Name       string
	Username   string
	PictureURL string
}

// PageNetwork is a network whose channels are pages chosen after authorizing.
type PageNetwork interface {
	Network
	Administered(ctx context.Context, accessToken string) ([]string, error)
	Page(ctx context.Context, accessToken, id string) (Page, error)
}

type OAuthStore interface {
	CreateAuthorization(ctx context.Context, a postgres.OAuthAuthorization, staleBefore time.Time) error
	TakeAuthorization(ctx context.Context, state string) (*postgres.OAuthAuthorization, error)
}

type OAuthChannels interface {
	ChannelStore
	Get(ctx context.Context, orgID, id uuid.UUID) (*postgres.Channel, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

// OAuth connects channels of the networks with OAuth (F5, F7).
type OAuth struct {
	Networks  map[string]Network
	Store     OAuthStore
	Channels  OAuthChannels
	Tokens    *tokens.Keeper
	Box       *secretbox.Box
	Files     storage.Files
	PublicURL string
	// LegacyCallbacks sends the networks Postiz's callback path.
	LegacyCallbacks bool
	Now             func() time.Time
	Logger          *slog.Logger
}

// Providers lists the networks that can be connected.
func (o *OAuth) Providers() []string {
	var out []string
	for name := range o.Networks {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// Where the networks send the browser back. postik serves both: its own
// path, and Postiz's, which is legacy but stays so the apps registered for
// suntzu keep working (S06 §2).
const (
	CallbackPrefix       = "/api/v1/channels/"
	CallbackSuffix       = "/callback"
	LegacyCallbackPrefix = "/integrations/social/"
)

// CallbackURL is the callback of provider on its own path or, when legacy,
// on Postiz's.
func (o *OAuth) CallbackURL(provider string, legacy bool) string {
	if legacy {
		return o.PublicURL + LegacyCallbackPrefix + provider
	}
	return o.PublicURL + CallbackPrefix + provider + CallbackSuffix
}

func (o *OAuth) redirect(provider string) string {
	return o.CallbackURL(provider, o.LegacyCallbacks)
}

// Start creates the authorization and returns where to send the browser.
// With channelID it reconnects that channel.
func (o *OAuth) Start(ctx context.Context, orgID, userID uuid.UUID, provider string, channelID *uuid.UUID) (string, error) {
	network, ok := o.Networks[provider]
	if !ok {
		return "", ErrUnknownProvider
	}
	if channelID != nil {
		channel, err := o.Channels.Get(ctx, orgID, *channelID)
		if errors.Is(err, postgres.ErrNotFound) || (err == nil && channel.Provider != provider) {
			return "", ErrChannelNotFound
		}
		if err != nil {
			return "", err
		}
	}
	begin, err := network.Begin(ctx, o.redirect(provider))
	if err != nil {
		return "", err
	}
	now := o.Now()
	auth := postgres.OAuthAuthorization{State: begin.State, OrganizationID: orgID, UserID: userID, Provider: provider, ChannelID: channelID, CreatedAt: now}
	if begin.Secret != nil {
		if auth.Secret, err = o.Box.Seal(begin.Secret, []byte(begin.State)); err != nil {
			return "", err
		}
	}
	if err := o.Store.CreateAuthorization(ctx, auth, now.Add(-oauth.AuthorizationTTL)); err != nil {
		return "", err
	}
	return begin.URL, nil
}

// Callback finishes an authorization for the member of the session and
// returns where to send the browser: /launches with added, continue or
// oauth_error (§3). redirect is the callback URL the browser came back to,
// which the network checks again when exchanging the grant.
func (o *OAuth) Callback(ctx context.Context, userID *uuid.UUID, provider string, q url.Values, redirect string) string {
	channelID, pending, code := o.callback(ctx, userID, provider, q, redirect)
	switch {
	case code != "":
		return "/launches?" + url.Values{"oauth_error": {code}}.Encode()
	case pending:
		return "/launches?" + url.Values{"continue": {channelID.String()}}.Encode()
	}
	return "/launches?" + url.Values{"added": {channelID.String()}}.Encode()
}

func (o *OAuth) callback(ctx context.Context, userID *uuid.UUID, provider string, q url.Values, redirect string) (uuid.UUID, bool, string) {
	network, ok := o.Networks[provider]
	if !ok {
		return uuid.Nil, false, oauth.ErrInvalidState
	}
	auth, err := o.take(ctx, network.State(q))
	if err != nil {
		o.Logger.ErrorContext(ctx, "take the authorization", "provider", provider, "error", err)
		return uuid.Nil, false, oauth.ErrInvalidState
	}
	if auth == nil {
		return uuid.Nil, false, oauth.ErrInvalidState
	}
	if auth.Provider != provider || userID == nil || auth.UserID != *userID {
		o.Logger.WarnContext(ctx, "an authorization came back to someone else", "provider", provider, "authorized_provider", auth.Provider, "authorized_user", auth.UserID, "user", userID)
		return uuid.Nil, false, oauth.ErrInvalidState
	}
	if network.Denied(q) {
		return uuid.Nil, false, oauth.ErrDenied
	}
	if oauth.Expired(auth.CreatedAt, o.Now()) {
		return uuid.Nil, false, oauth.ErrExpired
	}
	var secret []byte
	if auth.Secret != nil {
		if secret, err = o.Box.Open(auth.Secret, []byte(auth.State)); err != nil {
			o.Logger.ErrorContext(ctx, "open the authorization secret", "provider", provider, "error", err)
			return uuid.Nil, false, oauth.ErrExchangeFailed
		}
	}
	account, err := network.Finish(ctx, q, secret, redirect)
	if errors.Is(err, ErrMissingPermissions) {
		return uuid.Nil, false, oauth.ErrMissingPermissions
	}
	if err != nil {
		o.Logger.WarnContext(ctx, "finish the authorization", "provider", provider, "error", err)
		return uuid.Nil, false, oauth.ErrExchangeFailed
	}
	if auth.ChannelID != nil {
		id, code, err := o.reconnect(ctx, network, auth, account)
		if err != nil {
			o.Logger.ErrorContext(ctx, "reconnect the channel", "provider", provider, "error", err)
			return uuid.Nil, false, oauth.ErrExchangeFailed
		}
		return id, false, code
	}
	channel, err := o.connect(ctx, auth.OrganizationID, provider, network, account)
	if err != nil {
		o.Logger.ErrorContext(ctx, "connect the channel", "provider", provider, "error", err)
		return uuid.Nil, false, oauth.ErrExchangeFailed
	}
	return channel.ID, channel.InBetweenSteps, ""
}

func (o *OAuth) take(ctx context.Context, state string) (*postgres.OAuthAuthorization, error) {
	if state == "" {
		return nil, nil
	}
	return o.Store.TakeAuthorization(ctx, state)
}

// connect creates the channel of the account or updates the one the
// organization already has (§6.3, uniqueness).
func (o *OAuth) connect(ctx context.Context, orgID uuid.UUID, provider string, network Network, account Account) (*postgres.Channel, error) {
	externalID := account.ExternalID
	if account.Pending {
		externalID = oauth.PendingPage(account.ExternalID)
	}
	channel, err := o.Channels.FindByExternal(ctx, orgID, provider, externalID)
	if err != nil {
		return nil, err
	}
	now := o.Now()
	isNew := channel == nil
	if isNew {
		channel = &postgres.Channel{
			ID:             uuid.New(),
			OrganizationID: orgID,
			Provider:       provider,
			ExternalID:     externalID,
			PostingTimes:   append([]int(nil), channels.DefaultPostingTimes...),
			CreatedAt:      now,
		}
	}
	channel.Name = account.Name
	channel.Username = account.Username
	channel.RefreshNeeded = false
	channel.InBetweenSteps = account.Pending
	channel.UpdatedAt = now
	o.refreshPicture(ctx, network, channel, account.PictureURL)
	// The tokens go first so that a channel never looks connected without
	// them; a new channel has to exist before its tokens can.
	if !isNew {
		if err := o.Tokens.Save(ctx, channel.ID, account.Tokens); err != nil {
			return nil, err
		}
		return channel, o.Channels.Save(ctx, channel)
	}
	if err := o.Channels.Save(ctx, channel); err != nil {
		return nil, err
	}
	if err := o.Tokens.Save(ctx, channel.ID, account.Tokens); err != nil {
		if removeErr := o.Channels.Delete(ctx, orgID, channel.ID); removeErr != nil {
			o.Logger.ErrorContext(ctx, "remove a channel left without tokens", "channel", channel.ID, "error", removeErr)
		}
		o.removePicture(ctx, channel)
		return nil, err
	}
	return channel, nil
}

// reconnect gives a channel new tokens when the authorized account is the
// channel's (F7). For a page, the member has to administer it.
func (o *OAuth) reconnect(ctx context.Context, network Network, auth *postgres.OAuthAuthorization, account Account) (uuid.UUID, string, error) {
	channel, err := o.Channels.Get(ctx, auth.OrganizationID, *auth.ChannelID)
	if errors.Is(err, postgres.ErrNotFound) {
		return uuid.Nil, oauth.ErrInvalidState, nil
	}
	if err != nil {
		return uuid.Nil, "", err
	}
	if pages, ok := network.(PageNetwork); ok {
		administered, err := pages.Administered(ctx, account.Tokens.Access)
		if err != nil {
			return uuid.Nil, "", err
		}
		if !slices.Contains(administered, channel.ExternalID) {
			return uuid.Nil, oauth.ErrWrongAccount, nil
		}
	} else {
		if account.ExternalID != channel.ExternalID {
			return uuid.Nil, oauth.ErrWrongAccount, nil
		}
		channel.Name = account.Name
		channel.Username = account.Username
		o.refreshPicture(ctx, network, channel, account.PictureURL)
	}
	if err := o.Tokens.Save(ctx, channel.ID, account.Tokens); err != nil {
		return uuid.Nil, "", err
	}
	channel.RefreshNeeded = false
	channel.UpdatedAt = o.Now()
	return channel.ID, "", o.Channels.Save(ctx, channel)
}

func (o *OAuth) pageChannel(ctx context.Context, orgID, channelID uuid.UUID) (*postgres.Channel, PageNetwork, tokens.Tokens, error) {
	channel, err := o.Channels.Get(ctx, orgID, channelID)
	if errors.Is(err, postgres.ErrNotFound) {
		return nil, nil, tokens.Tokens{}, ErrChannelNotFound
	}
	if err != nil {
		return nil, nil, tokens.Tokens{}, err
	}
	network, ok := o.Networks[channel.Provider].(PageNetwork)
	if !ok || !channel.InBetweenSteps {
		return nil, nil, tokens.Tokens{}, ErrNotInBetweenSteps
	}
	t, err := o.Tokens.Load(ctx, channel.ID)
	return channel, network, t, err
}

// Pages lists the pages the member who connected the channel administers.
func (o *OAuth) Pages(ctx context.Context, orgID, channelID uuid.UUID) ([]Page, error) {
	_, network, t, err := o.pageChannel(ctx, orgID, channelID)
	if err != nil {
		return nil, err
	}
	ids, err := network.Administered(ctx, t.Access)
	if err != nil {
		return nil, err
	}
	pages := make([]Page, 0, len(ids))
	for _, id := range ids {
		page, err := network.Page(ctx, t.Access, id)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

// ChoosePage turns a channel in between steps into the page's channel. If
// the organization already had that page, that channel takes the tokens and
// the one in between steps goes (§3, step 5).
func (o *OAuth) ChoosePage(ctx context.Context, orgID, channelID uuid.UUID, pageID string) (*postgres.Channel, error) {
	channel, network, t, err := o.pageChannel(ctx, orgID, channelID)
	if err != nil {
		return nil, err
	}
	administered, err := network.Administered(ctx, t.Access)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(administered, pageID) {
		return nil, ErrPageNotAdministered
	}
	page, err := network.Page(ctx, t.Access, pageID)
	if err != nil {
		return nil, err
	}
	existing, err := o.Channels.FindByExternal(ctx, orgID, channel.Provider, pageID)
	if err != nil {
		return nil, err
	}
	now := o.Now()
	if existing != nil {
		existing.RefreshNeeded = false
		existing.UpdatedAt = now
		if err := o.Channels.Save(ctx, existing); err != nil {
			return nil, err
		}
		if err := o.Tokens.Save(ctx, existing.ID, t); err != nil {
			return nil, err
		}
		if err := o.Channels.Delete(ctx, orgID, channel.ID); err != nil {
			return nil, err
		}
		o.removePicture(ctx, channel)
		return o.Channels.Get(ctx, orgID, existing.ID)
	}
	channel.ExternalID = page.ID
	channel.Name = page.Name
	channel.Username = page.Username
	channel.InBetweenSteps = false
	channel.UpdatedAt = now
	o.refreshPicture(ctx, network, channel, page.PictureURL)
	if err := o.Channels.Save(ctx, channel); err != nil {
		return nil, err
	}
	return o.Channels.Get(ctx, orgID, channel.ID)
}

// refreshPicture replaces the stored picture; if it cannot be downloaded,
// the channel keeps the previous one.
func (o *OAuth) refreshPicture(ctx context.Context, network Network, channel *postgres.Channel, pictureURL string) {
	if pictureURL == "" {
		return
	}
	data, err := network.Picture(ctx, pictureURL)
	if err != nil {
		o.Logger.WarnContext(ctx, "download the channel picture", "channel", channel.ID, "error", err)
		return
	}
	path, err := o.Files.SaveAvatar(channel.ID, data, o.Now())
	if err != nil {
		o.Logger.WarnContext(ctx, "store the channel picture", "channel", channel.ID, "error", err)
		return
	}
	o.removePicture(ctx, channel)
	channel.Picture = &path
}

func (o *OAuth) removePicture(ctx context.Context, channel *postgres.Channel) {
	if channel.Picture == nil {
		return
	}
	if err := o.Files.Remove(*channel.Picture); err != nil {
		o.Logger.WarnContext(ctx, "remove the old channel picture", "path", *channel.Picture, "error", err)
	}
}
