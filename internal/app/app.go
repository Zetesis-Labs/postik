package app

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/uptrace/bun"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/auth"
	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/connect"
	"github.com/zetesis-labs/postik/internal/core/access"
	"github.com/zetesis-labs/postik/internal/email"
	"github.com/zetesis-labs/postik/internal/httpapi"
	"github.com/zetesis-labs/postik/internal/jobs"
	"github.com/zetesis-labs/postik/internal/library"
	"github.com/zetesis-labs/postik/internal/linkedin"
	"github.com/zetesis-labs/postik/internal/notify"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/secretbox"
	"github.com/zetesis-labs/postik/internal/storage"
	"github.com/zetesis-labs/postik/internal/telegram"
	"github.com/zetesis-labs/postik/internal/tokens"
	"github.com/zetesis-labs/postik/internal/webui"
)

type Deps struct {
	Config config.Config
	DB     *bun.DB
	Now    func() time.Time
	WebUI  fs.FS
	Logger *slog.Logger
}

// App is postik's HTTP handler and the jobs that run next to it.
type App struct {
	Handler http.Handler
	Jobs    *jobs.Jobs
}

func New(d Deps) (*App, error) {
	sessions := &auth.Sessions{
		Store:         postgres.NewSessions(d.DB),
		Now:           d.Now,
		SecureCookies: d.Config.SecureCookies(),
		Logger:        d.Logger,
	}
	identityStore := postgres.NewIdentity(d.DB)
	channelStore := postgres.NewChannels(d.DB)
	files := storage.Files{Dir: d.Config.StorageDir}
	var bot *telegram.Client
	if d.Config.Telegram != nil {
		bot = telegram.New(d.Config.Telegram.APIURL, d.Config.Telegram.BotToken)
	}
	notificationStore := postgres.NewNotifications(d.DB)
	notices := &notify.Service{Store: notificationStore, PublicURL: d.Config.PublicURL.String(), Now: d.Now, Logger: d.Logger}
	if d.Config.Email != nil {
		notices.Email = email.NewResend(d.Config.Email.APIURL, d.Config.Email.APIKey, d.Config.Email.From)
	}
	networks, err := newNetworks(d, channelStore, files)
	if err != nil {
		return nil, err
	}
	work, err := jobs.New(jobs.Deps{
		DB:       d.DB,
		Store:    postgres.NewPublications(d.DB),
		Telegram: bot,
		LinkedIn: networks.linkedIn,
		Tokens:   networks.tokens,
		Channels: channelStore,
		Files:    files,
		Notifier: notices,
		Digester: notices,
		Now:      d.Now,
		Logger:   d.Logger,
	})
	if err != nil {
		return nil, err
	}
	postStore := postgres.NewPosts(d.DB)
	postStore.Scheduler = work
	api := &httpapi.Server{
		Superadmin:    d.Config.Superadmin,
		OIDC:          d.Config.OIDC,
		Sessions:      sessions,
		Identity:      identityStore,
		Channels:      channelStore,
		Files:         files,
		Media:         &library.Library{Store: postgres.NewMediaStore(d.DB), Dir: d.Config.StorageDir, Now: d.Now},
		Posts:         postStore,
		Notifications: notificationStore,
		OAuth:         networks.oauth,
		Now:           d.Now,
		Logger:        d.Logger,
	}
	if d.Config.Telegram != nil {
		api.Telegram = &connect.Telegram{
			Client:   bot,
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
	if networks.oauth != nil {
		mux.HandleFunc("GET "+connect.CallbackPrefix+"{provider}"+connect.CallbackSuffix, oauthCallback(networks.oauth, false))
		mux.HandleFunc("GET "+connect.LegacyCallbackPrefix+"{provider}", oauthCallback(networks.oauth, true))
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

	return &App{Handler: crossOrigin.Handler(sessions.Middleware(limitUploads(mux))), Jobs: work}, nil
}

// networks is what postik needs to connect and use the networks with OAuth.
type networks struct {
	oauth    *connect.OAuth
	tokens   *tokens.Keeper
	linkedIn *linkedin.Client
}

func newNetworks(d Deps, channelStore *postgres.Channels, files storage.Files) (networks, error) {
	if d.Config.LinkedIn == nil {
		return networks{}, nil
	}
	box, err := secretbox.New(d.Config.EncryptionKey)
	if err != nil {
		return networks{}, err
	}
	li := d.Config.LinkedIn
	client := linkedin.New(li.AuthURL, li.APIURL, li.ClientID, li.ClientSecret, li.Version)
	profile := &connect.LinkedIn{Client: client, Now: d.Now}
	page := &connect.LinkedInPage{LinkedIn: *profile}
	keeper := &tokens.Keeper{
		Store:    postgres.NewCredentials(d.DB),
		Box:      box,
		Renewers: map[string]tokens.Renewer{"linkedin": profile, "linkedin-page": page},
		Now:      d.Now,
	}
	return networks{
		oauth: &connect.OAuth{
			Networks:        map[string]connect.Network{"linkedin": profile, "linkedin-page": page},
			Store:           postgres.NewOAuth(d.DB),
			Channels:        channelStore,
			Tokens:          keeper,
			Box:             box,
			Files:           files,
			PublicURL:       d.Config.PublicURL.String(),
			LegacyCallbacks: d.Config.LegacyCallbacks,
			Now:             d.Now,
			Logger:          d.Logger,
		},
		tokens:   keeper,
		linkedIn: client,
	}, nil
}

// oauthCallback is where the networks send the browser back (S06 §3), on
// postik's path or on Postiz's legacy one.
func oauthCallback(o *connect.OAuth, legacy bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userID *uuid.UUID
		if p, ok := auth.PrincipalFrom(r.Context()); ok && p.Kind == access.SessionMember {
			userID = p.UserID
		}
		provider := r.PathValue("provider")
		target := o.Callback(r.Context(), userID, provider, r.URL.Query(), o.CallbackURL(provider, legacy))
		http.Redirect(w, r, target, http.StatusFound)
	}
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
