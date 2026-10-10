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

# Run the TUI against a file or folder
run-tui path:
    go run ./cmd/ffconvert-tui '{{path}}'

# Run the web server against a library folder (default: current dir)
run-web library-dir='.':
    FF_CONVERT_LIBRARY_DIR='{{library-dir}}' go run ./cmd/ffconvert-web

# Run the desktop app in dev mode (requires the wails CLI)
run-desktop:
    cd cmd/ffconvert-desktop && ~/go/bin/wails dev -tags webkit2_41
