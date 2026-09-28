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
)

func main() {
	addr := flag.String("addr", ":5556", "listen address")
	issuer := flag.String("issuer", "http://localhost:5556", "public URL of the fake OIDC provider")
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	provider, err := fakeoidc.New(*issuer, time.Now)
	if err != nil {
		logger.Error("start fake OIDC provider", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("/", provider.Handler())

	logger.Info("postik-fakes is listening", "addr", *addr, "oidc_issuer", *issuer)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		logger.Error("serve", "error", err)
		os.Exit(1)
	}
}
