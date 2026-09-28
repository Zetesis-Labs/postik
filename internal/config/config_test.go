package config

import (
	"strings"
	"testing"
)

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":               "postgres://postik@localhost/postik",
		"POSTIK_PUBLIC_URL":          "https://postik.example",
		"POSTIK_SUPERADMIN_USERNAME": "admin",
		"POSTIK_SUPERADMIN_PASSWORD": "secret",
	}
}

func TestLoadValidConfiguration(t *testing.T) {
	env := validEnv()
	env["POSTIK_SUPERADMIN_TOTP_SECRET"] = "jbsw y3dp ehpk 3pxp"
	env["POSTIK_SUPERADMIN_RECOVERY_CODES"] = " alfa-1234 ,beta-5678,, "

	cfg, err := Load(envFrom(env))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
	if !cfg.SecureCookies() {
		t.Error("an https public URL must mark cookies as secure")
	}
	if !cfg.Superadmin.HasTOTP() {
		t.Error("the TOTP secret was not loaded")
	}
	if got := strings.Join(cfg.Superadmin.RecoveryCodes, "|"); got != "alfa-1234|beta-5678" {
		t.Errorf("RecoveryCodes = %q", got)
	}
	if cfg.Email != nil {
		t.Error("without a Resend key there is no email")
	}
}

func TestResendDefaultsToItsPublicAPI(t *testing.T) {
	env := validEnv()
	env["POSTIK_RESEND_API_KEY"] = "re_123"
	env["POSTIK_EMAIL_FROM"] = "postik <postik@mail.example>"
	cfg, err := Load(envFrom(env))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Email == nil || cfg.Email.APIURL != "https://api.resend.com" || cfg.Email.From != "postik <postik@mail.example>" {
		t.Fatalf("Email = %+v", cfg.Email)
	}
}

// S01.3 Una configuración incompleta impide arrancar y nombra la variable.
func TestIncompleteConfigurationNamesTheVariable(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(map[string]string)
		variable string
	}{
		{
			name:     "missing superadmin password",
			mutate:   func(env map[string]string) { delete(env, "POSTIK_SUPERADMIN_PASSWORD") },
			variable: "POSTIK_SUPERADMIN_PASSWORD",
		},
		{
			name:     "invalid TOTP secret",
			mutate:   func(env map[string]string) { env["POSTIK_SUPERADMIN_TOTP_SECRET"] = "not base32!" },
			variable: "POSTIK_SUPERADMIN_TOTP_SECRET",
		},
		{
			name:     "recovery codes without TOTP",
			mutate:   func(env map[string]string) { env["POSTIK_SUPERADMIN_RECOVERY_CODES"] = "alfa-1234" },
			variable: "POSTIK_SUPERADMIN_RECOVERY_CODES",
		},
		{
			name:     "missing database",
			mutate:   func(env map[string]string) { delete(env, "DATABASE_URL") },
			variable: "DATABASE_URL",
		},
		{
			name: "OIDC without client secret",
			mutate: func(env map[string]string) {
				env["POSTIK_OIDC_ISSUER"] = "https://auth.example/realms/zetesis"
				env["POSTIK_OIDC_CLIENT_ID"] = "postik"
			},
			variable: "POSTIK_OIDC_CLIENT_SECRET",
		},
		{
			name:     "Resend without sender",
			mutate:   func(env map[string]string) { env["POSTIK_RESEND_API_KEY"] = "re_123" },
			variable: "POSTIK_EMAIL_FROM",
		},
		{
			name:     "require invitation is not a boolean",
			mutate:   func(env map[string]string) { env["POSTIK_REQUIRE_INVITATION"] = "maybe" },
			variable: "POSTIK_REQUIRE_INVITATION",
		},
		{
			name:     "public URL without scheme",
			mutate:   func(env map[string]string) { env["POSTIK_PUBLIC_URL"] = "postik.example" },
			variable: "POSTIK_PUBLIC_URL",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			tc.mutate(env)
			_, err := Load(envFrom(env))
			if err == nil {
				t.Fatal("Load accepted an invalid configuration")
			}
			if !strings.Contains(err.Error(), tc.variable) {
				t.Errorf("error %q does not name %s", err, tc.variable)
			}
		})
	}
}

func TestLinkedInNeedsTheEncryptionKey(t *testing.T) {
	env := validEnv()
	env["POSTIK_LINKEDIN_CLIENT_ID"] = "client"
	env["POSTIK_LINKEDIN_CLIENT_SECRET"] = "secret"
	if _, err := Load(envFrom(env)); err == nil || !strings.Contains(err.Error(), "POSTIK_ENCRYPTION_KEY") {
		t.Fatalf("Load without key: %v", err)
	}

	env["POSTIK_ENCRYPTION_KEY"] = "c2hvcnQ="
	if _, err := Load(envFrom(env)); err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("Load with a short key: %v", err)
	}

	env["POSTIK_ENCRYPTION_KEY"] = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	cfg, err := Load(envFrom(env))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.EncryptionKey) != 32 || cfg.LinkedIn.Version != "202609" || cfg.LinkedIn.AuthURL != "https://www.linkedin.com" || cfg.LinkedIn.APIURL != "https://api.linkedin.com" {
		t.Fatalf("config = %+v, linkedin %+v", cfg, cfg.LinkedIn)
	}
}
