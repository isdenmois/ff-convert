# ff-convert

Batch movie transcoder wrapping ffmpeg/ffprobe. Converts a single movie file
or a folder of movie files (alphabetically) to MKV with:

- **video**: `copy` by default (remux), or per file AV1 (SVT-AV1 8-bit),
  HEVC (libx265 / NVENC / VAAPI), or any custom preset
- **audio**: every selected track → Opus 128k stereo, with a proper center-
  channel downmix (`pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5`
  for 5.1) and loudness normalization on by default
  (`loudnorm=i=-13:lra=15:tp=-1`, single-pass)
- **subtitles & attachments**: selected subtitle tracks and attachments
  (fonts, posters) are stream-copied; chapters are always kept
- outputs land in the output folder (`FF_CONVERT_OUTPUT_DIR`, by default
  `converted/` in the working directory) and are **never overwritten** —
  files that already have an output are skipped

Before converting, all files are probed with ffprobe and grouped by identical
audio+subtitle track layouts (language, title, codec, order, counts). Per
group you pick the tracks to keep (English is pre-selected; no English →
first audio track), loudnorm (on by default) and attachment handling; per
file you pick the video codec (`copy` by default).

Three frontends share one core:

| frontend | binary | input |
|---|---|---|
| terminal UI | `ffconvert-tui` | file/folder as argv |
| Linux desktop (Wails v2) | `ffconvert-desktop` | native file/folder dialogs, queue |
| web (chi + Vue SPA) | `ffconvert-web` | browse a library folder, queue, Docker-ready |

Web and desktop accept multiple queue items; all conversions share a worker
pool (default concurrency: CPU/2).

## Build & run

Requires: Go ≥1.27, ffmpeg + ffprobe in PATH, [bun](https://bun.sh) (for the
two Vue frontends). The desktop build additionally needs `gtk3` and
`webkit2gtk` (`webkit2_41` build tag when your distro ships 4.1).

```sh
# TUI + web binaries into bin/ (web SPA must be built first for the web UI)
just build

# TUI
go build -o ffconvert-tui ./cmd/ffconvert-tui
./ffconvert-tui /path/to/movie-or-folder

# web (SPA must be built first)
(cd web && bun install && bun run build)
go build -o ffconvert-web ./cmd/ffconvert-web
FF_CONVERT_LIBRARY_DIR=/path/to/movies ./ffconvert-web

# desktop (wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest)
cd cmd/ffconvert-desktop && wails build -tags webkit2_41
./build/bin/ffconvert-desktop
```

`just test` runs the test suite, `just format` gofmts the code.

Web development with hot reload: run `./ffconvert-web` (port 8080) and
`cd web && bun run dev` (Vite on 5173, proxying `/api`).

## Docker

```sh
docker compose -f docker-compose.example.yml up -d --build
```

Environment variables:

| variable | default | meaning |
|---|---|---|
| `FF_CONVERT_LIBRARY_DIR` | (required, web) | root folder browsable/convertible via the web UI |
| `FF_CONVERT_OUTPUT_DIR` | `./converted` | folder converted files are written to |
| `FF_CONVERT_PORT` | `8080` | web listen port |
| `FF_CONVERT_CONCURRENCY` | CPU/2 | worker pool size |
| `FF_CONVERT_CONFIG` | `~/.config/ff-convert/config.yaml` | preset config path |

Converted outputs are written into the output folder (`FF_CONVERT_OUTPUT_DIR`,
default `converted/` in the working directory), so mount it read-write.

## Configuration

See [`config.example.yaml`](config.example.yaml). Without a config, every
file's video codec is asked per task with `copy` preselected, loudnorm is
asked per group and preselected on, and attachments are preselected copy.
When a config exists, its `default_preset` is used for all files without
asking, and optional `audio.normalize` / `attachments` values pre-answer
those questions. Track selection is always interactive. Built-in presets:
`copy`, `av1` (SVT-AV1 p6 CRF 28 8-bit), `hevc` (libx265 CRF 23 8-bit),
`hevc-nvenc`, `hevc-vaapi`; custom presets are raw ffmpeg encoder args, so
hardware encoders can be configured freely.

## Layout

```
cmd/ffconvert-tui        Bubbletea terminal wizard
cmd/ffconvert-web        chi JSON API + WebSocket + embedded Vue SPA
cmd/ffconvert-desktop    Wails v2 app (bun-driven frontend)
internal/probe           ffprobe parsing
internal/scan            folder scanning (top-level, natural sort, skip done)
internal/group           strict metadata grouping
internal/plan            ffmpeg argument construction
internal/ffmpegx         ffmpeg runner with -progress parsing
internal/convert         orchestration (probe → ask → build → run)
internal/pool            bounded worker pool
internal/jobs            queue manager shared by web/desktop
internal/config          YAML presets + built-ins
web/                     Vue 3 + Vite + TS SPA (bun)
```

## Tests

```sh
go test ./...
```

Integration tests generate small fixtures with the local ffmpeg and are
skipped automatically when ffmpeg is unavailable.
