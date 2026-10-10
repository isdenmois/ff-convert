# syntax=docker/dockerfile:1

# --- Stage 1: build the SPA with bun ---
FROM oven/bun:1 AS spa
WORKDIR /app
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# --- Stage 2: build the Go binary (SPA embedded) ---
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=spa /app/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ffconvert-web ./cmd/ffconvert-web

# --- Stage 3: runtime with ffmpeg ---
FROM alpine:3.21
RUN apk add --no-cache ffmpeg ca-certificates tzdata
COPY --from=build /out/ffconvert-web /usr/local/bin/ffconvert-web
RUN adduser -D -u 1000 ffconvert
USER ffconvert
ENV FF_CONVERT_LIBRARY_DIR=/data \
    FF_CONVERT_PORT=8080
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["ffconvert-web"]
