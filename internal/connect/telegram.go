// Package connect links social accounts to channels of an organization.
package connect

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/core/channels"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/storage"
	"github.com/zetesis-labs/postik/internal/telegram"
)

const codeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var ErrConnectionNotFound = errors.New("telegram connection not found")

type TelegramStore interface {
	CreateConnection(ctx context.Context, conn postgres.TelegramConnection, staleBefore time.Time) (bool, error)
	Connection(ctx context.Context, code string) (*postgres.TelegramConnection, error)
	MarkConnected(ctx context.Context, code string, channelID uuid.UUID) error
	NextOffset(ctx context.Context) (int64, error)
	SaveNextOffset(ctx context.Context, offset int64) error
}

type ChannelStore interface {
	FindByExternal(ctx context.Context, orgID uuid.UUID, provider, externalID string) (*postgres.Channel, error)
	Save(ctx context.Context, channel *postgres.Channel) error
}

// Telegram connects chats with the /connect command (F6). Only one sync with
// Telegram runs at a time, because getUpdates allows a single consumer.
type Telegram struct {
	Client   *telegram.Client
	Store    TelegramStore
	Channels ChannelStore
	Files    storage.Files
	Now      func() time.Time
	Logger   *slog.Logger

	mu  sync.Mutex
	bot string
}

func (t *Telegram) BotUsername(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.botUsername(ctx)
}

func (t *Telegram) botUsername(ctx context.Context) (string, error) {
	if t.bot != "" {
		return t.bot, nil
	}
	me, err := t.Client.GetMe(ctx)
	if err != nil {
		return "", err
	}
	t.bot = me.Username
	return t.bot, nil
}

// Open creates a pending connection for the organization.
func (t *Telegram) Open(ctx context.Context, orgID uuid.UUID) (code, bot string, err error) {
	bot, err = t.BotUsername(ctx)
	if err != nil {
		return "", "", err
	}
	now := t.Now()
	for range 10 {
		code, err = randomCode()
		if err != nil {
			return "", "", err
		}
		created, err := t.Store.CreateConnection(ctx, postgres.TelegramConnection{Code: code, OrganizationID: orgID, CreatedAt: now}, now.Add(-channels.ConnectionTTL))
		if err != nil {
			return "", "", err
		}
		if created {
			return code, bot, nil
		}
	}
	return "", "", errors.New("no free connection code")
}

// Status syncs with Telegram and reports the connection of the organization.
func (t *Telegram) Status(ctx context.Context, orgID uuid.UUID, code string) (channels.ConnectionStatus, *uuid.UUID, error) {
	conn, err := t.Store.Connection(ctx, code)
	if err != nil {
		return "", nil, err
	}
	if conn == nil || conn.OrganizationID != orgID {
		return "", nil, ErrConnectionNotFound
	}
	if err := t.Sync(ctx); err != nil {
		return "", nil, err
	}
	if conn, err = t.Store.Connection(ctx, code); err != nil {
		return "", nil, err
	}
	return channels.ConnectionState(conn.CreatedAt, conn.ChannelID != nil, t.Now()), conn.ChannelID, nil
}

// Sync reads the pending updates and connects the chats whose /connect code
// belongs to a pending connection. An update that fails is retried next time.
func (t *Telegram) Sync(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	bot, err := t.botUsername(ctx)
	if err != nil {
		return err
	}
	offset, err := t.Store.NextOffset(ctx)
	if err != nil {
		return err
	}
	updates, err := t.Client.GetUpdates(ctx, offset)
	if err != nil {
		return err
	}
	for _, update := range updates {
		if post := update.Post(); post != nil {
			if code, ok := channels.ParseConnectCommand(post.Text, bot); ok {
				if err := t.resolve(ctx, code, post); err != nil {
					_ = t.Store.SaveNextOffset(ctx, update.UpdateID)
					return err
				}
			}
		}
		offset = update.UpdateID + 1
	}
	return t.Store.SaveNextOffset(ctx, offset)
}

func (t *Telegram) resolve(ctx context.Context, code string, post *telegram.Message) error {
	conn, err := t.Store.Connection(ctx, code)
	if err != nil {
		return err
	}
	now := t.Now()
	if conn == nil || channels.ConnectionState(conn.CreatedAt, conn.ChannelID != nil, now) != channels.Pending {
		return nil
	}
	chat, err := t.Client.GetChat(ctx, post.Chat.ID)
	if err != nil {
		return err
	}
	if err := t.Client.DeleteMessage(ctx, post.Chat.ID, post.MessageID); err != nil {
		t.Logger.InfoContext(ctx, "could not delete the /connect message", "chat", chat.ID, "error", err)
	}

	channel, err := t.Channels.FindByExternal(ctx, conn.OrganizationID, "telegram", strconv.FormatInt(chat.ID, 10))
	if err != nil {
		return err
	}
	if channel == nil {
		channel = &postgres.Channel{
			ID:             uuid.New(),
			OrganizationID: conn.OrganizationID,
			Provider:       "telegram",
			ExternalID:     strconv.FormatInt(chat.ID, 10),
			PostingTimes:   append([]int(nil), channels.DefaultPostingTimes...),
			CreatedAt:      now,
		}
	}
	channel.Name = channels.ChatName(channels.Chat{Type: chat.Type, Title: chat.Title, FirstName: chat.FirstName, LastName: chat.LastName})
	channel.Username = chat.Username
	channel.RefreshNeeded = false
	channel.InBetweenSteps = false
	channel.UpdatedAt = now
	t.refreshPicture(ctx, channel, chat, now)

	if err := t.Channels.Save(ctx, channel); err != nil {
		return err
	}
	return t.Store.MarkConnected(ctx, code, channel.ID)
}

// refreshPicture replaces the stored picture; if Telegram does not give one,
// the channel keeps the previous picture.
func (t *Telegram) refreshPicture(ctx context.Context, channel *postgres.Channel, chat telegram.Chat, now time.Time) {
	if chat.Photo == nil || chat.Photo.BigFileID == "" {
		return
	}
	data, err := t.Client.DownloadFile(ctx, chat.Photo.BigFileID)
	if err != nil {
		t.Logger.WarnContext(ctx, "download the chat picture", "chat", chat.ID, "error", err)
		return
	}
	path, err := t.Files.SaveAvatar(channel.ID, data, now)
	if err != nil {
		t.Logger.WarnContext(ctx, "store the chat picture", "chat", chat.ID, "error", err)
		return
	}
	if channel.Picture != nil {
		if err := t.Files.Remove(*channel.Picture); err != nil {
			t.Logger.WarnContext(ctx, "remove the old chat picture", "path", *channel.Picture, "error", err)
		}
	}
	channel.Picture = &path
}

func randomCode() (string, error) {
	code := make([]byte, 4)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			return "", err
		}
		code[i] = codeAlphabet[n.Int64()]
	}
	return string(code), nil
}
