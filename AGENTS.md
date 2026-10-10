# AGENTS.md

Batch movie transcoder wrapping ffmpeg/ffprobe → MKV (Opus 128k audio with
loudnorm, stream-copied subs/attachments, chapters kept). Three frontends share
one core: Bubbletea TUI, Wails v2 Linux desktop, chi + Vue web.

## Commands

- `just setup` — one-time: bun install in `web/` + lefthook git hooks
- `just build` — builds web SPA, then `bin/ffconvert-tui` and `bin/ffconvert-web`
- `just test` — `go test ./...` + `cd web && bun run test` (vitest)
- `just lint` — frontend only: `cd web && bun run lint` (biome)
- `just format` — `gofmt -w .` + `cd web && bun run format`
- Focused: `go test ./internal/plan/...`, `cd web && bun run typecheck` (vue-tsc)
- Desktop: `cd cmd/ffconvert-desktop && wails build -tags webkit2_41` (needs wails CLI)

Requires Go ≥1.27, ffmpeg/ffprobe in PATH, bun for both Vue frontends
(`web/` and `cmd/ffconvert-desktop/frontend/`). Desktop also needs gtk3 +
webkit2gtk; the `webkit2_41` build tag is used on distros shipping webkit 4.1.

## Layout & boundaries

```
cmd/ffconvert-tui        terminal wizard (bubbletea)
cmd/ffconvert-web        chi JSON API + WebSocket + embedded Vue SPA
cmd/ffconvert-desktop    Wails v2 app
internal/probe           ffprobe parsing
internal/scan            folder scan (top-level, natural sort, skip done)
internal/group           strict metadata grouping
internal/plan            ffmpeg argument construction
internal/ffmpegx         ffmpeg runner with -progress parsing
internal/convert         orchestration (probe → ask → build → run)
internal/pool            bounded worker pool
internal/jobs            queue manager shared by web/desktop
internal/config          YAML presets + built-ins
web/                     Vue 3 + Vite + TS SPA (bun, biome, vitest)
```

Core conversion logic lives in `internal/` and must stay frontend-agnostic;
frontends in `cmd/` only do I/O and presentation. Web and desktop share
`internal/jobs`; the TUI drives `internal/convert` directly.

## Conventions

- Go: gofmt; no dedicated Go linter configured.
- Frontend: biome (lint + format + imports), vue-tsc for typecheck, vitest.
- Pre-commit (lefthook, parallel): biome check --write + stage fixes,
  `bun run typecheck`, `bun run test` for staged frontend files; gofmt -l
  check and `go test ./...` for staged Go files. Run `just format` if gofmt
  fails.

## Gotchas

- The web SPA must be rebuilt (`just build` or `cd web && bun run build`)
  before `go build ./cmd/ffconvert-web` — the SPA is embedded.
- Integration tests generate fixtures with local ffmpeg and self-skip when
  ffmpeg is unavailable — don't assume a silent `go test` means they ran.
- Env vars: `FF_CONVERT_LIBRARY_DIR` (required, web), `FF_CONVERT_OUTPUT_DIR`
  (default `./converted`), `FF_CONVERT_PORT` (8080), `FF_CONVERT_CONCURRENCY`
  (CPU/2), `FF_CONVERT_CONFIG` (preset YAML path; see `config.example.yaml`).
- Converted outputs are never overwritten; already-converted inputs are
  skipped by the scanner.
- Web dev: run `./ffconvert-web` on 8080 + `cd web && bun run dev` (Vite on
  5173, proxies `/api`).
