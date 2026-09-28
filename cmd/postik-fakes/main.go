// Command postik-fakes serves the fake external services that postik talks
// to, for screen tests and for trying postik locally without real accounts.
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/zetesis-labs/postik/internal/testsupport/fakeoidc"
	"github.com/zetesis-labs/postik/internal/testsupport/fakeresend"
	"github.com/zetesis-labs/postik/internal/testsupport/faketelegram"
)

func main() {
	addr := flag.String("addr", ":5556", "listen address")
	issuer := flag.String("issuer", "http://localhost:5556", "public URL of the fake OIDC provider")
	botName := flag.String("bot", "postik_fake_bot", "username of the fake Telegram bot")
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	provider, err := fakeoidc.New(*issuer, time.Now)
	if err != nil {
		logger.Error("start fake OIDC provider", "error", err)
		os.Exit(1)
	}
	bot := faketelegram.New(*botName)
	mux := http.NewServeMux()
	mux.Handle("/telegram/", http.StripPrefix("/telegram", bot.Handler()))
	mux.Handle("/resend/", http.StripPrefix("/resend", fakeresend.New().Handler()))
	mux.Handle("/", provider.Handler())

	logger.Info("postik-fakes is listening", "addr", *addr, "oidc_issuer", *issuer, "telegram_api", *issuer+"/telegram", "telegram_token", faketelegram.Token, "resend_api", *issuer+"/resend", "resend_key", fakeresend.APIKey)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		logger.Error("serve", "error", err)
		os.Exit(1)
	}
}
