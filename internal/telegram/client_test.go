package telegram_test

import (
	"context"
	"errors"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/zetesis-labs/postik/internal/telegram"
	"github.com/zetesis-labs/postik/internal/testsupport/faketelegram"
)

func TestARefusedConnectionIsNotSent(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()

	client := telegram.New("http://"+address, faketelegram.Token)
	_, err = client.SendMessage(context.Background(), -1, "hola", 0)
	var notSent *telegram.NotSentError
	if !errors.As(err, &notSent) {
		t.Fatalf("err = %v, want NotSentError", err)
	}
}

func TestADroppedAnswerIsNotNotSent(t *testing.T) {
	bot := faketelegram.New("postik_bot")
	bot.SetChat(faketelegram.Chat{ID: -1, Type: "supergroup", Title: "G"})
	server := httptest.NewServer(bot.Handler())
	defer server.Close()
	bot.DropNext()

	_, err := telegram.New(server.URL, faketelegram.Token).SendMessage(context.Background(), -1, "hola", 0)
	var notSent *telegram.NotSentError
	if err == nil || errors.As(err, &notSent) {
		t.Fatalf("err = %v, want an error that is not NotSentError", err)
	}
}

func TestA429CarriesRetryAfter(t *testing.T) {
	bot := faketelegram.New("postik_bot")
	bot.SetChat(faketelegram.Chat{ID: -1, Type: "supergroup", Title: "G"})
	server := httptest.NewServer(bot.Handler())
	defer server.Close()
	bot.FailNext(429, "Too Many Requests: retry after 1")

	_, err := telegram.New(server.URL, faketelegram.Token).SendMessage(context.Background(), -1, "hola", 0)
	var apiErr *telegram.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 429 || apiErr.Parameters.RetryAfter != 1 {
		t.Fatalf("err = %#v", err)
	}
}
