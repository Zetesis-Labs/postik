package config

import (
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/zetesis-labs/postik/internal/core/access"
)

type Config struct {
	DatabaseURL       string
	PublicURL         *url.URL
	ListenAddr        string
	Superadmin        access.Superadmin
	OIDC              *OIDC
	RequireInvitation bool
	Telegram          *Telegram
	LinkedIn          *LinkedIn
	Email             *Email
	StorageDir        string
	// EncryptionKey seals the tokens of the networks (constitution §8).
	EncryptionKey []byte
	// LegacyCallbacks sends the networks Postiz's callback path (S06 §2).
	LegacyCallbacks bool
}

type LinkedIn struct {
	ClientID     string
	ClientSecret string
	Version      string
	AuthURL      string
	APIURL       string
}

// Email is Resend: without it postik sends no email.
type Email struct {
	APIKey string
	From   string
	APIURL string
}

type Telegram struct {
	BotToken string
	APIURL   string
}

type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	DisplayName  string
}

func (c Config) SecureCookies() bool {
	return c.PublicURL.Scheme == "https"
}

// PublicOrigin is scheme://host of the public URL, as browsers send it in Origin.
func (c Config) PublicOrigin() string {
	return c.PublicURL.Scheme + "://" + c.PublicURL.Host
}

// Load reads the configuration from getenv and names the offending variable on error.
func Load(getenv func(string) string) (Config, error) {
	var problems []error
	value := func(key string) string { return strings.TrimSpace(getenv(key)) }
	required := func(key string) string {
		v := value(key)
		if v == "" {
			problems = append(problems, fmt.Errorf("%s is required", key))
		}
		return v
	}

	cfg := Config{
		DatabaseURL: required("DATABASE_URL"),
		ListenAddr:  value("POSTIK_LISTEN_ADDR"),
		Superadmin: access.Superadmin{
			Username: required("POSTIK_SUPERADMIN_USERNAME"),
			Password: required("POSTIK_SUPERADMIN_PASSWORD"),
		},
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}

	if raw := required("POSTIK_PUBLIC_URL"); raw != "" {
		publicURL, err := parsePublicURL(raw)
		if err != nil {
			problems = append(problems, fmt.Errorf("POSTIK_PUBLIC_URL: %w", err))
		}
		cfg.PublicURL = publicURL
	}

	if raw := value("POSTIK_SUPERADMIN_TOTP_SECRET"); raw != "" {
		secret, err := decodeTOTPSecret(raw)
		if err != nil {
			problems = append(problems, fmt.Errorf("POSTIK_SUPERADMIN_TOTP_SECRET: %w", err))
		}
		cfg.Superadmin.TOTPSecret = secret
	}

	cfg.Superadmin.RecoveryCodes = splitList(value("POSTIK_SUPERADMIN_RECOVERY_CODES"))
	if len(cfg.Superadmin.RecoveryCodes) > 0 && value("POSTIK_SUPERADMIN_TOTP_SECRET") == "" {
		problems = append(problems, errors.New("POSTIK_SUPERADMIN_RECOVERY_CODES needs POSTIK_SUPERADMIN_TOTP_SECRET"))
	}

	cfg.StorageDir = value("POSTIK_STORAGE_DIR")
	if cfg.StorageDir == "" {
		cfg.StorageDir = "data"
	}
	if token := value("POSTIK_TELEGRAM_BOT_TOKEN"); token != "" {
		cfg.Telegram = &Telegram{BotToken: token, APIURL: strings.TrimRight(value("POSTIK_TELEGRAM_API_URL"), "/")}
		if cfg.Telegram.APIURL == "" {
			cfg.Telegram.APIURL = "https://api.telegram.org"
		}
	}

	linkedIn, linkedInProblems := loadLinkedIn(value)
	cfg.LinkedIn = linkedIn
	problems = append(problems, linkedInProblems...)

	if raw := value("POSTIK_ENCRYPTION_KEY"); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			problems = append(problems, errors.New("POSTIK_ENCRYPTION_KEY must be 32 bytes in base64"))
		}
		cfg.EncryptionKey = key
	} else if cfg.LinkedIn != nil {
		problems = append(problems, errors.New("POSTIK_ENCRYPTION_KEY is required when a network with OAuth is configured"))
	}

	if key := value("POSTIK_RESEND_API_KEY"); key != "" {
		cfg.Email = &Email{APIKey: key, From: value("POSTIK_EMAIL_FROM"), APIURL: strings.TrimRight(value("POSTIK_RESEND_API_URL"), "/")}
		if cfg.Email.From == "" {
			problems = append(problems, errors.New("POSTIK_EMAIL_FROM is required when POSTIK_RESEND_API_KEY is set"))
		}
		if cfg.Email.APIURL == "" {
			cfg.Email.APIURL = "https://api.resend.com"
		}
	}

	oidc, oidcProblems := loadOIDC(value)
	cfg.OIDC = oidc
	problems = append(problems, oidcProblems...)

	legacy, err := parseBool(value("POSTIK_LEGACY_CALLBACKS"))
	if err != nil {
		problems = append(problems, fmt.Errorf("POSTIK_LEGACY_CALLBACKS: %w", err))
	}
	cfg.LegacyCallbacks = legacy

	switch strings.ToLower(value("POSTIK_REQUIRE_INVITATION")) {
	case "", "false", "0", "no":
	case "true", "1", "yes":
		cfg.RequireInvitation = true
	default:
		problems = append(problems, errors.New("POSTIK_REQUIRE_INVITATION must be true or false"))
	}

	if len(problems) > 0 {
		return Config{}, errors.Join(problems...)
	}
	return cfg, nil
}

// loadLinkedIn returns nil when the app of LinkedIn is not configured. The
// client ID and secret go together.
func loadLinkedIn(value func(string) string) (*LinkedIn, []error) {
	id, secret := value("POSTIK_LINKEDIN_CLIENT_ID"), value("POSTIK_LINKEDIN_CLIENT_SECRET")
	if id == "" && secret == "" {
		return nil, nil
	}
	if id == "" || secret == "" {
		return nil, []error{errors.New("POSTIK_LINKEDIN_CLIENT_ID and POSTIK_LINKEDIN_CLIENT_SECRET go together")}
	}
	linkedIn := &LinkedIn{
		ClientID:     id,
		ClientSecret: secret,
		Version:      value("POSTIK_LINKEDIN_VERSION"),
		AuthURL:      strings.TrimRight(value("POSTIK_LINKEDIN_AUTH_URL"), "/"),
		APIURL:       strings.TrimRight(value("POSTIK_LINKEDIN_API_URL"), "/"),
	}
	if linkedIn.Version == "" {
		linkedIn.Version = "202609"
	}
	if linkedIn.AuthURL == "" {
		linkedIn.AuthURL = "https://www.linkedin.com"
	}
	if linkedIn.APIURL == "" {
		linkedIn.APIURL = "https://api.linkedin.com"
	}
	return linkedIn, nil
}

// loadOIDC returns nil when no provider is configured. The issuer, client ID
// and client secret go together.
func loadOIDC(value func(string) string) (*OIDC, []error) {
	oidc := &OIDC{
		Issuer:       strings.TrimRight(value("POSTIK_OIDC_ISSUER"), "/"),
		ClientID:     value("POSTIK_OIDC_CLIENT_ID"),
		ClientSecret: value("POSTIK_OIDC_CLIENT_SECRET"),
		DisplayName:  value("POSTIK_OIDC_DISPLAY_NAME"),
	}
	required := map[string]string{
		"POSTIK_OIDC_ISSUER":        oidc.Issuer,
		"POSTIK_OIDC_CLIENT_ID":     oidc.ClientID,
		"POSTIK_OIDC_CLIENT_SECRET": oidc.ClientSecret,
	}
	var missing []error
	set := 0
	for _, key := range []string{"POSTIK_OIDC_ISSUER", "POSTIK_OIDC_CLIENT_ID", "POSTIK_OIDC_CLIENT_SECRET"} {
		if required[key] == "" {
			missing = append(missing, fmt.Errorf("%s is required when OIDC is configured", key))
		} else {
			set++
		}
	}
	if set == 0 {
		return nil, nil
	}
	if len(missing) > 0 {
		return nil, missing
	}
	if oidc.DisplayName == "" {
		oidc.DisplayName = "OIDC"
	}
	return oidc, nil
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(raw) {
	case "", "false", "0", "no":
		return false, nil
	case "true", "1", "yes":
		return true, nil
	}
	return false, errors.New("must be true or false")
}

func parsePublicURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("must start with http:// or https://")
	}
	if parsed.Host == "" {
		return nil, errors.New("has no host")
	}
	return parsed, nil
}

func decodeTOTPSecret(raw string) ([]byte, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(raw, " ", ""))
	normalized = strings.TrimRight(normalized, "=")
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil {
		return nil, errors.New("is not valid base32")
	}
	if len(secret) < 10 {
		return nil, errors.New("is shorter than 80 bits")
	}
	return secret, nil
}

func splitList(raw string) []string {
	var items []string
	for item := range strings.SplitSeq(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
