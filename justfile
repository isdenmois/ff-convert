default:
    @just --list

# One-time setup after cloning: install frontend deps and git hooks
setup:
    cd web && bun install
    cd web && bunx lefthook install

# Build the TUI and web binaries into bin/
build:
    cd web && bun run build
    go build -o bin/ffconvert-tui ./cmd/ffconvert-tui
    go build -o bin/ffconvert-web ./cmd/ffconvert-web

# Run all tests (integration tests skip themselves without ffmpeg)
test:
    go test ./...
    cd web && bun run test

# Lint the frontend (biome: lint, format and import checks)
lint:
    cd web && bun run lint

# Format all code (Go + frontend)
format:
    gofmt -w .
    cd web && bun run format

# Run the TUI against a file or folder
run-tui path:
    go run ./cmd/ffconvert-tui '{{path}}'

# Run the web server against a library folder (default: current dir)
run-web library-dir='.':
    cd web && bun run build
    FF_CONVERT_LIBRARY_DIR='{{library-dir}}' go run ./cmd/ffconvert-web

# Run the desktop app in dev mode (requires the wails CLI)
run-desktop:
    cd cmd/ffconvert-desktop && ~/go/bin/wails dev -tags webkit2_41
