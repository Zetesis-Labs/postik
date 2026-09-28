.PHONY: generate fmt fmt-check vet test web-install web-typecheck web-build e2e spec-check migration migration-validate build run dev check

GO_DIRS := ./cmd ./internal ./tools

generate:
	go generate ./internal/httpapi
	cd web && pnpm generate:api

fmt:
	gofmt -w $(GO_DIRS)

fmt-check:
	@test -z "$$(gofmt -l $(GO_DIRS))" || { gofmt -l $(GO_DIRS); echo "Go files need formatting" >&2; exit 1; }

vet:
	go vet ./...

test:
	go test -race ./...

web-install:
	cd web && pnpm install --frozen-lockfile

web-typecheck:
	cd web && pnpm typecheck

web-build:
	cd web && pnpm build

e2e: web-build
	cd web && pnpm e2e

spec-check:
	go run ./tools/speccheck

migration:
	@test -n "$(name)" || { echo "usage: make migration name=add_channels" >&2; exit 1; }
	atlas migrate diff "$(name)" --env local

migration-validate:
	atlas migrate validate --env local

build: web-build
	CGO_ENABLED=0 go build -trimpath -o bin/postik ./cmd/postik

run: build
	./bin/postik migrate && ./bin/postik serve

dev: build
	./scripts/dev.sh

check: generate fmt-check vet migration-validate test web-typecheck spec-check
