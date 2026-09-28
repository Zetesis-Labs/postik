package app

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/uptrace/bun"

	"github.com/zetesis-labs/postik/internal/auth"
	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/connect"
	"github.com/zetesis-labs/postik/internal/httpapi"
	"github.com/zetesis-labs/postik/internal/library"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/storage"
	"github.com/zetesis-labs/postik/internal/telegram"
	"github.com/zetesis-labs/postik/internal/webui"
)

type Deps struct {
	Config config.Config
	DB     *bun.DB
	Now    func() time.Time
	WebUI  fs.FS
	Logger *slog.Logger
}

func New(d Deps) http.Handler {
	sessions := &auth.Sessions{
		Store:         postgres.NewSessions(d.DB),
		Now:           d.Now,
		SecureCookies: d.Config.SecureCookies(),
		Logger:        d.Logger,
	}
	identityStore := postgres.NewIdentity(d.DB)
	channelStore := postgres.NewChannels(d.DB)
	files := storage.Files{Dir: d.Config.StorageDir}
	api := &httpapi.Server{
		Superadmin: d.Config.Superadmin,
		OIDC:       d.Config.OIDC,
		Sessions:   sessions,
		Identity:   identityStore,
		Channels:   channelStore,
		Files:      files,
		Media:      &library.Library{Store: postgres.NewMediaStore(d.DB), Dir: d.Config.StorageDir, Now: d.Now},
		Now:        d.Now,
		Logger:     d.Logger,
	}
	if d.Config.Telegram != nil {
		api.Telegram = &connect.Telegram{
			Client:   telegram.New(d.Config.Telegram.APIURL, d.Config.Telegram.BotToken),
			Store:    postgres.NewTelegram(d.DB),
			Channels: channelStore,
			Files:    files,
			Now:      d.Now,
			Logger:   d.Logger,
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /readyz", readiness(d.DB))
	mux.Handle("GET "+storage.PublicPrefix, files.Handler())
	if d.Config.OIDC != nil {
		oidc := &auth.OIDC{
			Provider:          *d.Config.OIDC,
			RedirectURL:       d.Config.PublicURL.String() + auth.CallbackPath,
			RequireInvitation: d.Config.RequireInvitation,
			SecureCookies:     d.Config.SecureCookies(),
			Now:               d.Now,
			Logins:            postgres.NewOIDCLogins(d.DB),
			Identity:          identityStore,
			Sessions:          sessions,
			Logger:            d.Logger,
		}
		mux.HandleFunc("GET /api/v1/auth/oidc/login", oidc.Login)
		mux.HandleFunc("GET "+auth.CallbackPath, oidc.Callback)
	}
	httpapi.HandlerWithOptions(
		httpapi.NewStrictHandlerWithOptions(api, nil, httpapi.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  jsonError(d.Logger, http.StatusBadRequest, "bad_request"),
			ResponseErrorHandlerFunc: jsonError(d.Logger, http.StatusInternalServerError, "internal"),
		}),
		httpapi.StdHTTPServerOptions{
			BaseURL:          "/api/v1",
			BaseRouter:       mux,
			ErrorHandlerFunc: jsonError(d.Logger, http.StatusBadRequest, "bad_request"),
		},
	)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONError(w, http.StatusNotFound, "not_found", "Not found")
	})
	mux.Handle("/", webui.Handler(d.WebUI))

	crossOrigin := http.NewCrossOriginProtection()
	if err := crossOrigin.AddTrustedOrigin(d.Config.PublicOrigin()); err != nil {
		d.Logger.Error("trust public origin", "error", err)
	}
	crossOrigin.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSONError(w, http.StatusForbidden, "cross_origin", "Cross-origin request rejected")
	}))

	return crossOrigin.Handler(sessions.Middleware(limitUploads(mux)))
}

// maxUploadBody bounds a whole upload request: the largest video plus room
// for the multipart envelope.
const maxUploadBody = 1<<30 + 1<<20

func limitUploads(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/media" {
			r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)
		}
		next.ServeHTTP(w, r)
	})
}

func readiness(db *bun.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func jsonError(logger *slog.Logger, status int, code string) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		if status >= http.StatusInternalServerError {
			logger.ErrorContext(r.Context(), "request failed", "path", r.URL.Path, "error", err)
			writeJSONError(w, status, code, "Internal error")
			return
		}
		writeJSONError(w, status, code, err.Error())
	}
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}
