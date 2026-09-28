package config

import (
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/zetesis-labs/postik/internal/core/access"
)

type Config struct {
	DatabaseURL string
	PublicURL   *url.URL
	ListenAddr  string
	Superadmin  access.Superadmin
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

	if len(problems) > 0 {
		return Config{}, errors.Join(problems...)
	}
	return cfg, nil
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
