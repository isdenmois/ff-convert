# AGENTS.md

Batch movie transcoder wrapping ffmpeg/ffprobe → MKV (Opus 128k audio with
loudnorm, stream-copied subs/attachments, chapters kept). Three frontends share
one core: Bubbletea TUI, Wails v2 Linux desktop, chi + Vue web.

## Commands

- `just setup` — one-time: bun workspace install (root) + lefthook git hooks
- `just build` — builds web SPA, then `bin/ffconvert-tui` and `bin/ffconvert-web`
- `just test` — `go test ./...` + vitest in `packages/shared` and `web`
- `just lint` — frontends only: biome over `packages/`, `web/`, desktop frontend
- `just format` — `gofmt -w .` + biome format over the same frontend dirs
- Focused: `go test ./internal/plan/...`, `bun run --cwd web typecheck` (vue-tsc)
- Desktop: `cd cmd/ffconvert-desktop && wails build -tags webkit2_41` (needs wails CLI)

Requires Go ≥1.27, ffmpeg/ffprobe in PATH, bun for the frontend workspace
(root `package.json` links `packages/*`, `web/` and
`cmd/ffconvert-desktop/frontend/`; always `bun install` at the root — per-dir
installs are no longer a thing). Desktop also needs gtk3 + webkit2gtk; the
`webkit2_41` build tag is used on distros shipping webkit 4.1.

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
packages/shared          @ffconvert/shared — code shared by both Vue frontends
web/                     Vue 3 + Vite + TS SPA (bun, biome, vitest)
```

Core conversion logic lives in `internal/` and must stay frontend-agnostic;
frontends in `cmd/` only do I/O and presentation. Web and desktop share
`internal/jobs`; the TUI drives `internal/convert` directly.

### Frontend structure (Feature-Sliced Design, v2.1)

Both Vue frontends use FSD layers `app/` → `pages/` → `features/` →
`entities/` → `shared/` with Pinia (setup stores) registered in `app/`:

```
web/src/                              cmd/ffconvert-desktop/frontend/src/
  app/main.ts (pinia), app.css          app/main.ts (pinia), app.css
  pages/main/ (MainPage: composition)   pages/main/ (MainPage: toolbar
  features/                               composition + error line)
    library-browser/ (store + UI)      features/
    job-queue/ (queue UI)                add-job/ (store + UI)
    answers-dialog/ (modal UI)           output-dir/ (store + UI)
  entities/job/ (jobs store + WS          job-queue/, answers-dialog/
    feed: jobs, ws, answering)          entities/job/ (jobs store +
  shared/api/ (REST client, jobs,        EventsOn feed: jobs, answering)
    library)                           shared/api/ (wailsjs IPC wrappers:
                                          jobs, dialogs, output-dir)
                                        wailsjs/ is generated — never edit
```

The queue is the one domain model: `entities/job` holds the Pinia store
(JobView snapshots + feed, `orderedJobs`, answering/dismissed dialog
bookkeeping, queue actions wrapping `shared/api`). Features own their
interaction state in their own Pinia stores (library navigation, add-job,
output-dir) and consume `entities/job` only through its public `index.ts`;
features never import each other. Pages compose features and own
page-level error display.

`packages/shared` (`@ffconvert/shared`, consumed as `workspace:*`) holds the
transport-agnostic UI both frontends render (JobCard, AnswersModal), the job
DTO types, `formatDuration` and the global theme CSS. Its components take a
`submit` callback prop / emit instead of calling a transport — changes there
affect BOTH frontends. Import rule: `app → pages → features → entities →
shared` (+ the package); never import upward, across slices on the same
layer, or past a slice's `index.ts`.

## Conventions

- Go: gofmt; no dedicated Go linter configured.
- Frontend: biome (lint + format + imports, root `biome.json`), vue-tsc for
  typecheck, vitest.
- Pre-commit (lefthook, parallel): biome check --write + stage fixes,
  `bun run --cwd packages/shared typecheck && bun run --cwd web typecheck`,
  desktop typecheck when `wailsjs/` exists, vitest for staged frontend files;
  gofmt -l check and `go test ./...` for staged Go files. Run `just format`
  if gofmt fails.

## Gotchas

- The web SPA must be rebuilt (`just build` or `cd web && bun run build`)
  before `go build ./cmd/ffconvert-web` — the SPA is embedded.
- `@ffconvert/shared` is consumed as TS source through `workspace:*`; after
  changing it, typecheck/test BOTH frontends (`web` and the desktop frontend).
- `cmd/ffconvert-desktop/frontend/wailsjs/` is generated by the wails CLI and
  gitignored — without it the desktop frontend cannot typecheck (the pre-commit
  hook skips it in that case).
- Integration tests generate fixtures with local ffmpeg and self-skip when
  ffmpeg is unavailable — don't assume a silent `go test` means they ran.
- Env vars: `FF_CONVERT_LIBRARY_DIR` (required, web), `FF_CONVERT_OUTPUT_DIR`
  (default `./converted`), `FF_CONVERT_PORT` (8080), `FF_CONVERT_CONCURRENCY`
  (CPU/2), `FF_CONVERT_CONFIG` (preset YAML path; see `config.example.yaml`).
- Converted outputs are never overwritten; already-converted inputs are
  skipped by the scanner.
- Web dev: run `./ffconvert-web` on 8080 + `cd web && bun run dev` (Vite on
  5173, proxies `/api`).
