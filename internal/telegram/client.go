// Package telegram is a small client for the parts of the Telegram Bot API
// that postik uses.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	// Uploads sends files, which can take much longer than a plain call.
	Uploads *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 20 * time.Second},
		Uploads: &http.Client{Timeout: 10 * time.Minute},
	}
}

// APIError is an answer with "ok": false.
type APIError struct {
	Code        int    `json:"error_code"`
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram %d: %s", e.Code, e.Description)
}

// NotSentError means the request never reached Telegram: the connection
// could not be opened, so nothing can have been published.
type NotSentError struct {
	Err error
}

func (e *NotSentError) Error() string { return "telegram unreachable: " + e.Err.Error() }
func (e *NotSentError) Unwrap() error { return e.Err }

func notSent(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
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
	return c.do(c.HTTP, req, method, result)
}

func (c *Client) do(client *http.Client, req *http.Request, method string, result any) error {
	res, err := client.Do(req)
	if err != nil {
		if notSent(err) {
			return &NotSentError{Err: err}
		}
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

// File is a file to upload: Type is photo or video.
type File struct {
	Type string
	Name string
	Open func() (io.ReadCloser, error)
}

// SendMessage sends HTML text, as a reply when replyTo is not zero.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, replyTo int64) (Message, error) {
	params := map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML"}
	if replyTo != 0 {
		params["reply_to_message_id"] = replyTo
	}
	var message Message
	err := c.call(ctx, "sendMessage", params, &message)
	return message, err
}

// SendFile sends one photo or video with an HTML caption.
func (c *Client) SendFile(ctx context.Context, chatID int64, file File, caption string, replyTo int64) (Message, error) {
	fields := map[string]string{"chat_id": strconv.FormatInt(chatID, 10), "caption": caption, "parse_mode": "HTML"}
	if replyTo != 0 {
		fields["reply_to_message_id"] = strconv.FormatInt(replyTo, 10)
	}
	method := map[string]string{"photo": "sendPhoto", "video": "sendVideo"}[file.Type]
	var message Message
	err := c.upload(ctx, method, fields, map[string]File{file.Type: file}, &message)
	return message, err
}

// SendMediaGroup sends 2 to 10 files as an album, with the caption on the first one.
func (c *Client) SendMediaGroup(ctx context.Context, chatID int64, files []File, caption string, replyTo int64) ([]Message, error) {
	type inputMedia struct {
		Type      string `json:"type"`
		Media     string `json:"media"`
		Caption   string `json:"caption,omitempty"`
		ParseMode string `json:"parse_mode,omitempty"`
	}
	media := make([]inputMedia, len(files))
	attached := make(map[string]File, len(files))
	for i, f := range files {
		name := "file" + strconv.Itoa(i)
		media[i] = inputMedia{Type: f.Type, Media: "attach://" + name}
		if i == 0 && caption != "" {
			media[i].Caption, media[i].ParseMode = caption, "HTML"
		}
		attached[name] = f
	}
	encoded, err := json.Marshal(media)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{"chat_id": strconv.FormatInt(chatID, 10), "media": string(encoded)}
	if replyTo != 0 {
		fields["reply_to_message_id"] = strconv.FormatInt(replyTo, 10)
	}
	var messages []Message
	err = c.upload(ctx, "sendMediaGroup", fields, attached, &messages)
	return messages, err
}

// upload streams a multipart/form-data request, so large videos are not held in memory.
func (c *Client) upload(ctx context.Context, method string, fields map[string]string, files map[string]File, result any) error {
	body, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	go func() {
		writer.CloseWithError(writeForm(form, fields, files))
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/bot"+c.Token+"/"+method, body)
	if err != nil {
		_ = body.Close()
		return err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	return c.do(c.Uploads, req, method, result)
}

func writeForm(form *multipart.Writer, fields map[string]string, files map[string]File) error {
	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			return err
		}
	}
	for field, f := range files {
		part, err := form.CreateFormFile(field, f.Name)
		if err != nil {
			return err
		}
		content, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(part, content)
		_ = content.Close()
		if err != nil {
			return err
		}
	}
	return form.Close()
}
