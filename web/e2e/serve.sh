#!/usr/bin/env bash
# Arranca el binario de postik y sus servicios falsos contra una base de datos
# desechable para Playwright.
set -euo pipefail

port="${POSTIK_E2E_PORT:-8090}"
fakes_port="${POSTIK_E2E_FAKES_PORT:-5557}"
admin_url="${TEST_DATABASE_URL:?TEST_DATABASE_URL is required}"
database="postik_e2e"

psql "$admin_url" -q -c "DROP DATABASE IF EXISTS ${database} WITH (FORCE)" -c "CREATE DATABASE ${database}"
database_url="${admin_url%/*}/${database}?sslmode=disable"

cd "$(dirname "$0")/../.."
go build -o bin/postik ./cmd/postik
go build -o bin/postik-fakes ./cmd/postik-fakes

./bin/postik-fakes -addr ":${fakes_port}" -issuer "http://localhost:${fakes_port}" &
fakes=$!
trap 'kill "$fakes" 2>/dev/null || true' EXIT INT TERM

export DATABASE_URL="$database_url"
export POSTIK_PUBLIC_URL="http://localhost:${port}"
export POSTIK_LISTEN_ADDR=":${port}"
export POSTIK_SUPERADMIN_USERNAME="admin"
export POSTIK_SUPERADMIN_PASSWORD="e2e-password"
export POSTIK_SUPERADMIN_TOTP_SECRET="JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"
export POSTIK_SUPERADMIN_RECOVERY_CODES="alfa-1234"
export POSTIK_OIDC_ISSUER="http://localhost:${fakes_port}"
export POSTIK_OIDC_CLIENT_ID="postik"
export POSTIK_OIDC_CLIENT_SECRET="postik-secret"
export POSTIK_OIDC_DISPLAY_NAME="Fake"
export POSTIK_TELEGRAM_BOT_TOKEN="fake-token"
export POSTIK_TELEGRAM_API_URL="http://localhost:${fakes_port}/telegram"
export POSTIK_STORAGE_DIR="$(mktemp -d)"
export POSTIK_RESEND_API_KEY="re_fake"
export POSTIK_RESEND_API_URL="http://localhost:${fakes_port}/resend"
export POSTIK_EMAIL_FROM="postik <postik@example.com>"
POSTIK_ENCRYPTION_KEY="$(head -c 32 /dev/urandom | base64)"
export POSTIK_ENCRYPTION_KEY
export POSTIK_LINKEDIN_CLIENT_ID="fake-linkedin-client"
export POSTIK_LINKEDIN_CLIENT_SECRET="fake-linkedin-secret"
export POSTIK_LINKEDIN_AUTH_URL="http://localhost:${fakes_port}/linkedin"
export POSTIK_LINKEDIN_API_URL="http://localhost:${fakes_port}/linkedin"

./bin/postik migrate
./bin/postik serve &
wait $!
