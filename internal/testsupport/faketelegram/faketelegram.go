// Package faketelegram imitates the parts of the Telegram Bot API that postik
// uses, for tests and local development.
package faketelegram

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const Token = "fake-token"

// Chat is a group, channel or private chat the bot can see.
type Chat struct {
	ID        int64
	Type      string
	Title     string
	Username  string
	FirstName string
	LastName  string
	Photo     []byte
	BotAdmin  bool
}

type update struct {
	UpdateID    int64           `json:"update_id"`
	Message     json.RawMessage `json:"message,omitempty"`
	ChannelPost json.RawMessage `json:"channel_post,omitempty"`
}

type Bot struct {
	Username string

	mu        sync.Mutex
	chats     map[int64]*Chat
	updates   []update
	nextID    int64
	messageID int64
	deleted   map[int64][]int64
	sent      []Sent
	failure   *failure
}

// Sent is a message the bot was asked to send.
type Sent struct {
	Method    string      `json:"method"`
	ChatID    int64       `json:"chatId"`
	Text      string      `json:"text"`
	ParseMode string      `json:"parseMode"`
	ReplyTo   int64       `json:"replyTo"`
	Media     []SentMedia `json:"media"`
	MessageID int64       `json:"messageId"`
}

// SentMedia is a file that came with a message. Caption is only set inside
// media groups; in sendPhoto and sendVideo it is the message's Text.
type SentMedia struct {
	Type    string `json:"type"`
	Caption string `json:"caption"`
	Name    string `json:"name"`
	Data    []byte `json:"-"`
	Size    int    `json:"size"`
}

// failure is what the next send answers instead of sending.
type failure struct {
	code        int
	description string
	drop        bool
}

func New(username string) *Bot {
	return &Bot{Username: username, chats: map[int64]*Chat{}, nextID: 1, messageID: 100, deleted: map[int64][]int64{}}
}

// SetChat adds or replaces a chat.
func (b *Bot) SetChat(chat Chat) {
	b.mu.Lock()
	defer b.mu.Unlock()
	copied := chat
	b.chats[chat.ID] = &copied
}

// Say queues text as a message in the chat and returns its message ID.
// Channels deliver it as a channel post.
func (b *Bot) Say(chatID int64, text string) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	chat := b.chats[chatID]
	b.messageID++
	message, _ := json.Marshal(map[string]any{
		"message_id": b.messageID,
		"chat":       chatJSON(chat),
		"text":       text,
	})
	u := update{UpdateID: b.nextID}
	if chat.Type == "channel" {
		u.ChannelPost = message
	} else {
		u.Message = message
	}
	b.updates = append(b.updates, u)
	b.nextID++
	return b.messageID
}

// Deleted lists the messages the bot deleted in a chat.
func (b *Bot) Deleted(chatID int64) []int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]int64(nil), b.deleted[chatID]...)
}

// Sent lists, in order, the messages the bot sent.
func (b *Bot) Sent() []Sent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Sent(nil), b.sent...)
}

// FailNext makes the next send answer with an error instead of sending. A 429
// carries retry_after.
func (b *Bot) FailNext(code int, description string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failure = &failure{code: code, description: description}
}

// DropNext makes the next send read the request and close the connection
// without answering, so the caller cannot know whether it went out.
func (b *Bot) DropNext() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failure = &failure{drop: true}
}

func chatJSON(c *Chat) map[string]any {
	out := map[string]any{"id": c.ID, "type": c.Type}
	if c.Title != "" {
		out["title"] = c.Title
	}
	if c.Username != "" {
		out["username"] = c.Username
	}
	if c.FirstName != "" {
		out["first_name"] = c.FirstName
	}
	if c.LastName != "" {
		out["last_name"] = c.LastName
	}
	return out
}

// Handler serves the Bot API at /bot<token>/<method>, files at
// /file/bot<token>/<path> and the controls used by Playwright and people.
func (b *Bot) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/bot"+Token+"/{method}", b.method)
	mux.HandleFunc("GET /file/bot"+Token+"/photos/{chat}", b.file)
	mux.HandleFunc("POST /_control/message", b.controlMessage)
	mux.HandleFunc("POST /_control/fail", b.controlFail)
	mux.HandleFunc("GET /_control/sent", b.controlSent)
	mux.HandleFunc("GET /_control", b.controlForm)
	return mux
}

var sendMethods = map[string]bool{"sendMessage": true, "sendPhoto": true, "sendVideo": true, "sendDocument": true, "sendMediaGroup": true}

func (b *Bot) method(w http.ResponseWriter, r *http.Request) {
	if sendMethods[r.PathValue("method")] {
		b.send(w, r)
		return
	}
	_ = r.ParseForm()
	params := r.Form
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var body map[string]any
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		_ = decoder.Decode(&body)
		for k, v := range body {
			params.Set(k, fmt.Sprint(v))
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch r.PathValue("method") {
	case "getMe":
		ok(w, map[string]any{"id": 1, "is_bot": true, "username": b.Username})
	case "getUpdates":
		offset, _ := strconv.ParseInt(params.Get("offset"), 10, 64)
		var pending []update
		for _, u := range b.updates {
			if u.UpdateID >= offset {
				pending = append(pending, u)
			}
		}
		b.updates = pending
		if pending == nil {
			pending = []update{}
		}
		ok(w, pending)
	case "getChat":
		chat, found := b.chat(params.Get("chat_id"))
		if !found {
			fail(w, "Bad Request: chat not found")
			return
		}
		out := chatJSON(chat)
		if chat.Photo != nil {
			out["photo"] = map[string]string{"small_file_id": "small", "big_file_id": fmt.Sprintf("big-%d", chat.ID)}
		}
		ok(w, out)
	case "getFile":
		chatID := strings.TrimPrefix(params.Get("file_id"), "big-")
		ok(w, map[string]string{"file_id": params.Get("file_id"), "file_path": "photos/" + chatID})
	case "deleteMessage":
		chat, found := b.chat(params.Get("chat_id"))
		if !found || !chat.BotAdmin {
			fail(w, "Bad Request: message can't be deleted")
			return
		}
		messageID, _ := strconv.ParseInt(params.Get("message_id"), 10, 64)
		b.deleted[chat.ID] = append(b.deleted[chat.ID], messageID)
		ok(w, true)
	default:
		fail(w, "Not Found: method not found")
	}
}

// send handles the send* methods, which come as JSON or, with files, as
// multipart/form-data.
func (b *Bot) send(w http.ResponseWriter, r *http.Request) {
	method := r.PathValue("method")
	params, files, err := readSend(r)
	if err != nil {
		fail(w, "Bad Request: "+err.Error())
		return
	}
	b.mu.Lock()
	failure := b.failure
	b.failure = nil
	b.mu.Unlock()
	if failure != nil {
		if failure.drop {
			if hijacker, ok := w.(http.Hijacker); ok {
				if conn, _, err := hijacker.Hijack(); err == nil {
					_ = conn.Close()
					return
				}
			}
			panic(http.ErrAbortHandler)
		}
		failWith(w, failure.code, failure.description)
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	chat, found := b.chat(params["chat_id"])
	if !found {
		fail(w, "Bad Request: chat not found")
		return
	}
	sent := Sent{Method: method, ChatID: chat.ID, ParseMode: params["parse_mode"]}
	sent.ReplyTo, _ = strconv.ParseInt(params["reply_to_message_id"], 10, 64)
	switch method {
	case "sendMessage":
		sent.Text = params["text"]
	case "sendPhoto", "sendVideo", "sendDocument":
		sent.Text = params["caption"]
		field := map[string]string{"sendPhoto": "photo", "sendVideo": "video", "sendDocument": "document"}[method]
		file, ok := files[field]
		if !ok {
			fail(w, "Bad Request: there is no "+field+" in the request")
			return
		}
		sent.Media = []SentMedia{{Type: field, Name: file.Name, Data: file.Data, Size: len(file.Data)}}
	case "sendMediaGroup":
		var items []struct {
			Type      string `json:"type"`
			Media     string `json:"media"`
			Caption   string `json:"caption"`
			ParseMode string `json:"parse_mode"`
		}
		if err := json.Unmarshal([]byte(params["media"]), &items); err != nil {
			fail(w, "Bad Request: can't parse media JSON object")
			return
		}
		for _, item := range items {
			file, ok := files[strings.TrimPrefix(item.Media, "attach://")]
			if !ok {
				fail(w, "Bad Request: wrong file identifier/HTTP URL specified")
				return
			}
			sent.Media = append(sent.Media, SentMedia{Type: item.Type, Caption: item.Caption, Name: file.Name, Data: file.Data, Size: len(file.Data)})
			if item.Caption != "" && sent.ParseMode == "" {
				sent.ParseMode = item.ParseMode
			}
		}
	}

	count := max(len(sent.Media), 1)
	if method != "sendMediaGroup" {
		count = 1
	}
	var messages []map[string]any
	for range count {
		b.messageID++
		messages = append(messages, map[string]any{"message_id": b.messageID, "chat": chatJSON(chat), "date": 0})
	}
	sent.MessageID = messages[0]["message_id"].(int64)
	b.sent = append(b.sent, sent)
	if method == "sendMediaGroup" {
		ok(w, messages)
		return
	}
	ok(w, messages[0])
}

type upload struct {
	Name string
	Data []byte
}

func readSend(r *http.Request) (map[string]string, map[string]upload, error) {
	params := map[string]string{}
	files := map[string]upload{}
	switch {
	case strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"):
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			return nil, nil, err
		}
		for k, v := range r.MultipartForm.Value {
			params[k] = v[0]
		}
		for field, headers := range r.MultipartForm.File {
			f, err := headers[0].Open()
			if err != nil {
				return nil, nil, err
			}
			data, err := io.ReadAll(f)
			_ = f.Close()
			if err != nil {
				return nil, nil, err
			}
			files[field] = upload{Name: headers[0].Filename, Data: data}
		}
	case strings.HasPrefix(r.Header.Get("Content-Type"), "application/json"):
		var body map[string]any
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&body); err != nil {
			return nil, nil, err
		}
		for k, v := range body {
			params[k] = fmt.Sprint(v)
		}
	default:
		if err := r.ParseForm(); err != nil {
			return nil, nil, err
		}
		for k, v := range r.Form {
			params[k] = v[0]
		}
	}
	return params, files, nil
}

func (b *Bot) chat(raw string) (*Chat, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, false
	}
	chat, found := b.chats[id]
	return chat, found
}

func (b *Bot) file(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	chat, found := b.chat(r.PathValue("chat"))
	b.mu.Unlock()
	if !found || chat.Photo == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(chat.Photo)
}

// controlMessage creates the chat if needed and says text in it:
// {"chatId": -100, "title": "Grupo", "type": "supergroup", "text": "/connect abcd"}.
func (b *Bot) controlMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChatID int64  `json:"chatId"`
		Title  string `json:"title"`
		Type   string `json:"type"`
		Text   string `json:"text"`
	}
	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		_ = r.ParseForm()
		body.ChatID, _ = strconv.ParseInt(r.PostForm.Get("chatId"), 10, 64)
		body.Title, body.Type, body.Text = r.PostForm.Get("title"), r.PostForm.Get("type"), r.PostForm.Get("text")
	}
	if body.Type == "" {
		body.Type = "supergroup"
	}
	b.mu.Lock()
	_, known := b.chats[body.ChatID]
	b.mu.Unlock()
	if !known {
		b.SetChat(Chat{ID: body.ChatID, Type: body.Type, Title: body.Title, BotAdmin: true})
	}
	b.Say(body.ChatID, body.Text)
	w.WriteHeader(http.StatusNoContent)
}

// controlFail prepares the next send to fail: {"code": 400, "description": "Bad Request: chat not found"}
// or {"drop": true}.
func (b *Bot) controlFail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code        int    `json:"code"`
		Description string `json:"description"`
		Drop        bool   `json:"drop"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Drop {
		b.DropNext()
	} else {
		b.FailNext(body.Code, body.Description)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (b *Bot) controlSent(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sent := b.Sent()
	if sent == nil {
		sent = []Sent{}
	}
	_ = json.NewEncoder(w).Encode(sent)
}

var controlPage = template.Must(template.New("control").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Fake Telegram</title>
<style>body{font-family:sans-serif;background:#0e0e0e;color:#fff;display:flex;justify-content:center;padding-top:80px}
form{display:flex;flex-direction:column;gap:12px;width:360px}input{padding:8px}button{padding:10px;background:#2EA6DD;color:#fff;border:0}</style>
</head><body><form method="post" action="_control/message" onsubmit="setTimeout(()=>this.reset(),50)">
<h1>Fake Telegram · @{{.}}</h1>
<label>Chat ID <input name="chatId" value="-1001"></label>
<label>Chat title <input name="title" value="Grupo de pruebas"></label>
<label>Message <input name="text" placeholder="/connect abcd" required></label>
<button type="submit">Send</button>
</form></body></html>`))

func (b *Bot) controlForm(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = controlPage.Execute(w, b.Username)
}

func ok(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
}

func fail(w http.ResponseWriter, description string) {
	failWith(w, http.StatusBadRequest, description)
}

func failWith(w http.ResponseWriter, code int, description string) {
	body := map[string]any{"ok": false, "error_code": code, "description": description}
	if code == http.StatusTooManyRequests {
		body["parameters"] = map[string]any{"retry_after": 1}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
