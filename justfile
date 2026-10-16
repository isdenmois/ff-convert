default:
    @just --list

# One-time setup after cloning: install workspace deps and git hooks
setup:
    bun install
    cd web && bunx lefthook install
    cd web && bunx playwright install chromium

# Build the TUI and web binaries into bin/
build:
    cd web && bun run build
    go build -o bin/ffconvert-tui ./cmd/ffconvert-tui
    go build -o bin/ffconvert-web ./cmd/ffconvert-web

# Run all tests (integration tests skip themselves without ffmpeg)
test:
    go test ./...
    cd packages/shared && bun run test
    cd web && bun run test
    cd cmd/ffconvert-desktop/frontend && bun run typecheck

# Lint the frontends (biome: lint, format and import checks)
lint:
    bunx biome check packages web cmd/ffconvert-desktop/frontend

# Run the Playwright e2e suite (mock API server + Vite dev server)
e2e:
    cd web && bun run e2e

# Format all code (Go + frontends)
format:
    gofmt -w .
    bunx biome format --write packages web cmd/ffconvert-desktop/frontend

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

package:
    cd packaging && makepkg -srci

