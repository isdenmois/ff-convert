# syntax=docker/dockerfile:1

# --- Stage 1: build the SPA with bun (root workspace install) ---
FROM oven/bun:1.4.3-alpine AS spa
WORKDIR /app
# manifests first for layer caching; the lockfile covers ALL workspace
# members, so every member manifest must be present for --frozen-lockfile
COPY package.json bun.lock ./
COPY packages/shared/package.json packages/shared/
COPY web/package.json web/
COPY cmd/ffconvert-desktop/frontend/package.json cmd/ffconvert-desktop/frontend/
RUN bun install --frozen-lockfile
# sources; **/node_modules is dockerignored so the install above survives
COPY packages/shared/ packages/shared/
COPY web/ web/
RUN bun run --cwd web build

# --- Stage 2: build the Go binary (SPA embedded) ---
FROM golang:1.27-alpine3.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
COPY --from=spa /app/web/dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o /out/ffconvert-web ./cmd/ffconvert-web

# --- Stage 3: runtime with ffmpeg ---
FROM alpine:3.24
LABEL org.opencontainers.image.title="ffconvert" \
      org.opencontainers.image.source="https://github.com/isden/ff-convert" \
      org.opencontainers.image.description="Batch movie transcoder wrapping ffmpeg → MKV (web UI)" \
      org.opencontainers.image.licenses="MIT"
ARG PUID=1000
ARG PGID=1000
RUN apk add --no-cache ffmpeg \
 && addgroup -g ${PGID} ffconvert \
 && adduser -D -u ${PUID} -G ffconvert ffconvert \
 && mkdir -p /data && chown ffconvert:ffconvert /data
USER ffconvert
WORKDIR /data
ENV FF_CONVERT_LIBRARY_DIR=/data \
    FF_CONVERT_OUTPUT_DIR=/result \
    FF_CONVERT_CONFIG=/config/config.yaml \
    FF_CONVERT_PORT=8080
VOLUME /data
VOLUME /result
VOLUME /config
EXPOSE 8080
COPY --from=build /out/ffconvert-web /usr/local/bin/ffconvert-web
ENTRYPOINT ["ffconvert-web"]
