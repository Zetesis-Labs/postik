// Package tokens keeps the tokens of the channels sealed in the database and
// renews them when the network allows it (S06 §4).
package tokens

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/core/oauth"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/secretbox"
)

// Tokens are what a channel uses to act on its network.
type Tokens struct {
	Access           string
	Secret           string
	Refresh          string
	ExpiresAt        *time.Time
	RefreshExpiresAt *time.Time
}

var (
	// ErrCannotRenew means the channel has to be reconnected: there is no
	// refresh token or the network refused it.
	ErrCannotRenew = errors.New("the token cannot be renewed")
	// ErrNoCredentials means the channel was never given tokens.
	ErrNoCredentials = errors.New("the channel has no credentials")
)

// Renewer asks a network for new tokens. It wraps ErrCannotRenew when the
// network refuses the refresh token.
type Renewer interface {
	Renew(ctx context.Context, refresh string) (Tokens, error)
}

type Keeper struct {
	Store    *postgres.Credentials
	Box      *secretbox.Box
	Renewers map[string]Renewer
	Now      func() time.Time
}

// Save seals and stores the tokens of a channel.
func (k *Keeper) Save(ctx context.Context, channelID uuid.UUID, t Tokens) error {
	c, err := k.seal(channelID, t)
	if err != nil {
		return err
	}
	return k.Store.Save(ctx, c)
}

// Load returns the stored tokens of a channel as they are.
func (k *Keeper) Load(ctx context.Context, channelID uuid.UUID) (Tokens, error) {
	c, err := k.Store.Get(ctx, channelID)
	if err != nil {
		return Tokens{}, err
	}
	if c == nil {
		return Tokens{}, ErrNoCredentials
	}
	return k.open(c)
}

// Current returns tokens ready to use, renewing them first when they are
// about to expire and can be renewed.
func (k *Keeper) Current(ctx context.Context, channel postgres.Channel) (Tokens, error) {
	t, err := k.Load(ctx, channel.ID)
	if err != nil {
		return Tokens{}, err
	}
	if !oauth.NeedsRenewal(t.ExpiresAt, t.Refresh != "", k.Now()) {
		return t, nil
	}
	return k.Renew(ctx, channel, t.Access)
}

// Renew replaces tokens the network rejected. stale is the access token that
// was rejected: if another job already replaced it, the new one is returned
// without asking the network again.
func (k *Keeper) Renew(ctx context.Context, channel postgres.Channel, stale string) (Tokens, error) {
	renewer, ok := k.Renewers[channel.Provider]
	if !ok {
		return Tokens{}, ErrCannotRenew
	}
	var renewed Tokens
	err := k.Store.Update(ctx, channel.ID, func(current *postgres.Credential) (*postgres.Credential, error) {
		t, err := k.open(current)
		if err != nil {
			return nil, err
		}
		if t.Access != stale {
			renewed = t
			return nil, nil
		}
		if t.Refresh == "" {
			return nil, ErrCannotRenew
		}
		next, err := renewer.Renew(ctx, t.Refresh)
		if err != nil {
			return nil, err
		}
		if next.Refresh == "" {
			next.Refresh, next.RefreshExpiresAt = t.Refresh, t.RefreshExpiresAt
		}
		renewed = next
		return k.seal(channel.ID, next)
	})
	if errors.Is(err, postgres.ErrNotFound) {
		return Tokens{}, ErrNoCredentials
	}
	return renewed, err
}

func associated(channelID uuid.UUID, field string) []byte {
	return []byte(channelID.String() + ":" + field)
}

func (k *Keeper) seal(channelID uuid.UUID, t Tokens) (*postgres.Credential, error) {
	c := &postgres.Credential{ChannelID: channelID, ExpiresAt: t.ExpiresAt, RefreshExpiresAt: t.RefreshExpiresAt, UpdatedAt: k.Now()}
	var err error
	if c.AccessToken, err = k.Box.Seal([]byte(t.Access), associated(channelID, "access")); err != nil {
		return nil, err
	}
	if t.Secret != "" {
		if c.AccessSecret, err = k.Box.Seal([]byte(t.Secret), associated(channelID, "secret")); err != nil {
			return nil, err
		}
	}
	if t.Refresh != "" {
		if c.RefreshToken, err = k.Box.Seal([]byte(t.Refresh), associated(channelID, "refresh")); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (k *Keeper) open(c *postgres.Credential) (Tokens, error) {
	t := Tokens{ExpiresAt: c.ExpiresAt, RefreshExpiresAt: c.RefreshExpiresAt}
	fields := []struct {
		sealed []byte
		name   string
		into   *string
	}{
		{c.AccessToken, "access", &t.Access},
		{c.AccessSecret, "secret", &t.Secret},
		{c.RefreshToken, "refresh", &t.Refresh},
	}
	for _, f := range fields {
		if f.sealed == nil {
			continue
		}
		plain, err := k.Box.Open(f.sealed, associated(c.ChannelID, f.name))
		if err != nil {
			return Tokens{}, fmt.Errorf("open the %s token of channel %s: %w", f.name, c.ChannelID, err)
		}
		*f.into = string(plain)
	}
	return t, nil
}
