// Package telegram is a small client for the parts of the Telegram Bot API
// that postik uses.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{BaseURL: baseURL, Token: token, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// APIError is an answer with "ok": false.
type APIError struct {
	Code        int    `json:"error_code"`
	Description string `json:"description"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram %d: %s", e.Code, e.Description)
}

func (c *Client) call(ctx context.Context, method string, params map[string]any, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/bot"+c.Token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	defer res.Body.Close()
	var envelope struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		APIError
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("telegram %s: decode: %w", method, err)
	}
	if !envelope.OK {
		return &envelope.APIError
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(envelope.Result, result)
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func (c *Client) GetMe(ctx context.Context) (User, error) {
	var me User
	err := c.call(ctx, "getMe", map[string]any{}, &me)
	return me, err
}

type Chat struct {
	ID        int64      `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Username  string     `json:"username"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	Photo     *ChatPhoto `json:"photo"`
}

type ChatPhoto struct {
	BigFileID string `json:"big_file_id"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type Update struct {
	UpdateID    int64    `json:"update_id"`
	Message     *Message `json:"message"`
	ChannelPost *Message `json:"channel_post"`
}

// Post is the message or channel post the update carries, if any.
func (u Update) Post() *Message {
	if u.Message != nil {
		return u.Message
	}
	return u.ChannelPost
}

// GetUpdates returns the updates from offset on, without waiting, and
// confirms the ones before offset.
func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         0,
		"allowed_updates": []string{"message", "channel_post"},
	}, &updates)
	return updates, err
}

func (c *Client) GetChat(ctx context.Context, chatID int64) (Chat, error) {
	var chat Chat
	err := c.call(ctx, "getChat", map[string]any{"chat_id": chatID}, &chat)
	return chat, err
}

func (c *Client) DeleteMessage(ctx context.Context, chatID, messageID int64) error {
	return c.call(ctx, "deleteMessage", map[string]any{"chat_id": chatID, "message_id": messageID}, nil)
}

// DownloadFile returns the contents of a file by its ID.
func (c *Client) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	var file struct {
		FilePath string `json:"file_path"`
	}
	if err := c.call(ctx, "getFile", map[string]any{"file_id": fileID}, &file); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/file/bot"+c.Token+"/"+file.FilePath, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram file %s: status %d", file.FilePath, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 10<<20))
}
