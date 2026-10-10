default:
    @just --list

# Build the TUI and web binaries into bin/
build:
    go build -o bin/ffconvert-tui ./cmd/ffconvert-tui
    go build -o bin/ffconvert-web ./cmd/ffconvert-web

# Run all tests (integration tests skip themselves without ffmpeg)
test:
    go test ./...

# Format all Go code
format:
    gofmt -w .
