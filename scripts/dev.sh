#!/usr/bin/env bash
# Arranca postik y los servicios falsos (OIDC) para probar en local.
set -euo pipefail
cd "$(dirname "$0")/.."

go build -o bin/postik-fakes ./cmd/postik-fakes
./bin/postik-fakes -addr :5556 -issuer http://localhost:5556 &
fakes=$!
trap 'kill "$fakes" 2>/dev/null || true' EXIT INT TERM

./bin/postik migrate
./bin/postik serve
