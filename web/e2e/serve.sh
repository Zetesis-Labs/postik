#!/usr/bin/env bash
# Arranca el binario de postik contra una base de datos desechable para Playwright.
set -euo pipefail

port="${POSTIK_E2E_PORT:-8090}"
admin_url="${TEST_DATABASE_URL:?TEST_DATABASE_URL is required}"
database="postik_e2e"

psql "$admin_url" -q -c "DROP DATABASE IF EXISTS ${database} WITH (FORCE)" -c "CREATE DATABASE ${database}"
database_url="${admin_url%/*}/${database}?sslmode=disable"

cd "$(dirname "$0")/../.."
go build -o bin/postik ./cmd/postik

export DATABASE_URL="$database_url"
export POSTIK_PUBLIC_URL="http://localhost:${port}"
export POSTIK_LISTEN_ADDR=":${port}"
export POSTIK_SUPERADMIN_USERNAME="admin"
export POSTIK_SUPERADMIN_PASSWORD="e2e-password"
export POSTIK_SUPERADMIN_TOTP_SECRET="JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"
export POSTIK_SUPERADMIN_RECOVERY_CODES="alfa-1234"

./bin/postik migrate
exec ./bin/postik serve
