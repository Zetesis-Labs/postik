# syntax=docker/dockerfile:1.7

FROM node:24-bookworm-slim AS web
RUN npm install -g pnpm@10.18.0
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.26-bookworm AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/postik ./cmd/postik

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/postik /usr/local/bin/postik
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/postik"]
CMD ["serve"]
