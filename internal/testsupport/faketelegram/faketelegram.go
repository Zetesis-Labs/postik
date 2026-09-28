// Package faketelegram imitates the parts of the Telegram Bot API that postik
// uses, for tests and local development.
package faketelegram

import (
	"encoding/json"
	"fmt"
	"html/template"
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
	mux.HandleFunc("GET /_control", b.controlForm)
	return mux
}

func (b *Bot) method(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 400, "description": description})
}
