import type {
  Answers,
  GroupView,
  JobView,
  LibraryEntry,
  QuestionsView,
  StreamView,
} from "../../src/types";

/**
 * Stateful mock of the ffconvert-web backend. Mirrors the real API contract
 * (cmd/ffconvert-web/main.go + internal/jobs) closely enough to drive the SPA:
 * the job lifecycle advances on timers so tests can await real UI outcomes
 * (progress bar, ETA, state badges) instead of hand-rolled WS frame timelines.
 */

/** Error carrying an HTTP status, translated to `{"error": ...}` by the server. */
export class MockError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

// ---------------------------------------------------------------------------
// Library
// ---------------------------------------------------------------------------

/** Nested library tree: a dir maps to an object, a file maps to `true`. */
export interface LibraryTree {
  [name: string]: LibraryTree | true;
}

function resolveDir(tree: LibraryTree, path: string): LibraryTree | null {
  let node: LibraryTree | true = tree;
  for (const part of path.split("/").filter(Boolean)) {
    if (typeof node !== "object" || !(part in node)) return null;
    node = node[part];
  }
  return typeof node === "object" ? node : null;
}

function listEntries(tree: LibraryTree, path: string): LibraryEntry[] {
  const dir = resolveDir(tree, path);
  if (!dir) {
    throw new MockError(403, "path escapes library root");
  }
  const entries: LibraryEntry[] = Object.entries(dir).map(([name, node]) => ({
    name,
    path: [path, name].filter(Boolean).join("/"),
    dir: typeof node === "object",
  }));
  // Real backend: dirs first, case-insensitive name sort.
  entries.sort(
    (a, b) =>
      Number(b.dir) - Number(a.dir) ||
      a.name.localeCompare(b.name, undefined, { sensitivity: "base" }),
  );
  return entries;
}

// ---------------------------------------------------------------------------
// Job templates
// ---------------------------------------------------------------------------

export interface StreamSpec {
  language?: string;
  codec?: string;
  channels?: number;
  layout?: string;
  title?: string;
}

export interface GroupSpec {
  /** File names sharing this track layout (default: one "<group>.mkv"). */
  files?: string[];
  audio?: StreamSpec[];
  subs?: StreamSpec[];
  /** Positions within `audio` preselected by default (default: first track). */
  audioDefaults?: number[];
  subsDefaults?: number[];
}

export interface JobTemplate {
  /** Library-relative path of the converted file or folder. */
  path: string;
  name?: string;
  groups?: GroupSpec[];
  ask?: { preset?: boolean; normalize?: boolean; attachments?: boolean };
  presetChoices?: string[];
  defaultPreset?: string;
  normalizeDefault?: boolean;
  attachmentsDefault?: boolean;
  /** ms in `probing` before `questions` appear (default 300). */
  probeMs?: number;
  /** ms in `queued` before `running` starts (default 200). */
  startMs?: number;
  /** progress tick interval (default 150). */
  tickMs?: number;
  /** fraction gained per tick by the active file (default 0.1). */
  progressPerTick?: number;
  /** delays the answers POST response, to exercise the modal busy state. */
  answerDelayMs?: number;
  /** when set, submitting answers fails with this message (after answerDelayMs). */
  answersError?: string;
  /** positions within the file list whose conversion fails at ~70%. */
  failFiles?: number[];
  skipped?: string[];
}

const DEFAULT_AUDIO: StreamSpec[] = [
  { language: "eng", codec: "ac3", channels: 6, layout: "5.1" },
  { language: "rus", codec: "ac3", channels: 6, layout: "5.1" },
];
const DEFAULT_SUBS: StreamSpec[] = [{ language: "eng", codec: "srt" }];

function toStream(spec: StreamSpec, index: number, kind: "audio" | "subs"): StreamView {
  const parts = [
    spec.language ?? "und",
    (spec.codec ?? (kind === "audio" ? "ac3" : "srt")).toUpperCase(),
    spec.layout ?? (spec.channels === 6 ? "5.1" : spec.channels === 2 ? "stereo" : ""),
    spec.title ?? "",
  ].filter(Boolean);
  return {
    index,
    label: `  ${parts.join("  ")}`.trim(),
    language: spec.language ?? "und",
    title: spec.title,
    codec: spec.codec,
    channels: spec.channels,
    layout: spec.layout,
  };
}

function buildQuestions(t: JobTemplate): QuestionsView {
  const filesOf = defaultFiles(t.path);
  const groups: GroupView[] = (t.groups ?? [{}]).map((g, i) => {
    const files = g.files ?? filesOf(i);
    const audio = (g.audio ?? DEFAULT_AUDIO).map((s, j) => toStream(s, j + 1, "audio"));
    const subs = (g.subs ?? DEFAULT_SUBS).map((s, j) => toStream(s, audio.length + j + 1, "subs"));
    const firstAudio = audio.at((g.audioDefaults ?? [0])[0] ?? 0) ?? audio[0];
    return {
      key: `g${i + 1}`,
      file_count: files.length,
      files,
      tasks: files.map((name) => ({ path: taskPath(t.path, name), name })),
      audio,
      subs,
      audio_defaults: firstAudio ? [firstAudio.index] : [],
      subs_defaults: (g.subsDefaults ?? [])
        .map((p) => (subs[p] ?? subs[0])?.index)
        .filter((x) => x !== undefined),
    };
  });
  return {
    groups,
    preset_choices: t.presetChoices ?? ["copy", "av1", "hevc", "hevc-nvenc", "hevc-vaapi"],
    ask_preset: t.ask?.preset ?? true,
    ask_normalize: t.ask?.normalize ?? true,
    ask_attachments: t.ask?.attachments ?? true,
    default_preset: t.defaultPreset ?? "copy",
    defaults: {
      audio: [],
      subs: [],
      preset: t.defaultPreset ?? "copy",
      normalize: t.normalizeDefault ?? true,
      attachments: t.attachmentsDefault ?? true,
    },
  };
}

function basename(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? path;
}

/** Extensions recognized by the real scanner (internal/scan). */
const VIDEO_EXT = /\.(mkv|mp4|m4v|avi|mov|ts|webm|wmv|flv)$/i;

/** A job on a video file has exactly that file; a folder job joins group file names. */
function taskPath(jobPath: string, fileName: string): string {
  if (VIDEO_EXT.test(jobPath)) return jobPath;
  return [jobPath, fileName].filter(Boolean).join("/");
}

/** Default file names per group when a template omits them: the basename for the
 *  single-group case, de-duplicated with a suffix for extra groups. */
function defaultFiles(path: string): (i: number) => string[] {
  return (i: number) => [basename(path) + (i > 0 ? `-g${i + 1}` : "")];
}

// ---------------------------------------------------------------------------
// Job lifecycle
// ---------------------------------------------------------------------------

interface MockJob {
  view: JobView;
  template: JobTemplate;
  filePaths: string[];
  timers: ReturnType<typeof setTimeout>[];
  ticker?: ReturnType<typeof setInterval>;
  runningSince?: number;
}

export interface RecordedRequest {
  method: string;
  path: string;
  body?: unknown;
}

const TERMINAL = new Set(["done", "failed", "canceled"]);

export class MockState {
  private jobs: MockJob[] = [];
  private tree: LibraryTree = {};
  private nextId = 1;
  readonly requests: RecordedRequest[] = [];

  /** Invoked after every state mutation; the WS layer broadcasts a snapshot. */
  onChange: () => void = () => {};

  reset(): void {
    for (const job of this.jobs) {
      for (const t of job.timers) clearTimeout(t);
      if (job.ticker) clearInterval(job.ticker);
    }
    this.jobs = [];
    this.tree = {};
    this.nextId = 1;
    this.requests.length = 0;
  }

  // -- library ---------------------------------------------------------------

  setLibrary(tree: LibraryTree): void {
    this.tree = tree;
  }

  listLibrary(path: string): LibraryEntry[] {
    return listEntries(this.tree, path);
  }

  // -- jobs ------------------------------------------------------------------

  /**
   * Create a job. `initialState: "questions"` skips the probing delay so
   * modal-focused specs don't need to wait for the timer.
   */
  create(template: JobTemplate, initialState?: "questions"): JobView {
    const id = String(this.nextId++);
    const filesOf = defaultFiles(template.path);
    const filePaths = VIDEO_EXT.test(template.path)
      ? [template.path]
      : (template.groups ?? [{}]).flatMap((g, i) =>
          (g.files ?? filesOf(i)).map((name) => taskPath(template.path, name)),
        );
    const view: JobView = {
      id,
      path: template.path,
      name: template.name ?? basename(template.path),
      state: "probing",
      created_at: new Date().toISOString(),
      skipped: template.skipped?.map((name) => ({
        path: taskPath(template.path, name),
        name,
        output: `${name}.mkv`,
      })),
      overall: 0,
      done: 0,
      failed: 0,
      total: filePaths.length,
    };
    const job: MockJob = { view, template, filePaths, timers: [] };
    this.jobs.push(job);
    const questions = buildQuestions(template);
    if (initialState === "questions") {
      view.state = "questions";
      view.questions = questions;
    } else {
      job.timers.push(
        setTimeout(() => {
          view.state = "questions";
          view.questions = questions;
          this.onChange();
        }, template.probeMs ?? 300),
      );
    }
    this.onChange();
    return structuredClone(view);
  }

  list(): JobView[] {
    return structuredClone(this.jobs.map((j) => j.view));
  }

  get(id: string): JobView {
    const job = this.jobs.find((j) => j.view.id === id);
    if (!job) throw new MockError(404, `job ${id} not found`);
    return structuredClone(job.view);
  }

  async submitAnswers(id: string, answers: Answers[]): Promise<JobView> {
    const job = this.mustFind(id, "questions");
    const t = job.template;
    if (t.answerDelayMs) {
      // Exercise the modal's busy state before resolving (or rejecting).
      await new Promise((resolve) => setTimeout(resolve, t.answerDelayMs));
    }
    if (t.answersError) throw new MockError(400, t.answersError);
    const groups = job.view.questions?.groups ?? [];
    if (answers.length !== groups.length) {
      throw new MockError(400, `expected ${groups.length} answers, got ${answers.length}`);
    }
    job.view.files = job.filePaths.map((p) => {
      const name = p.split("/").at(-1) ?? p;
      return {
        path: p,
        name,
        output: `${name.replace(/\.[^.]+$/, "")}.mkv`,
        state: "pending",
        percent: 0,
      };
    });
    job.view.state = "queued";
    delete job.view.questions;
    this.onChange();
    job.timers.push(setTimeout(() => this.start(job), t.startMs ?? 200));
    return structuredClone(job.view);
  }

  cancel(id: string): JobView {
    const job = this.mustFind(id);
    if (TERMINAL.has(job.view.state)) throw new MockError(400, `job ${id} already finished`);
    this.stopTimers(job);
    job.view.state = "canceled";
    for (const f of job.view.files ?? []) {
      if (f.state === "pending" || f.state === "running") f.state = "canceled";
    }
    this.onChange();
    return structuredClone(job.view);
  }

  remove(id: string): void {
    const job = this.jobs.find((j) => j.view.id === id);
    if (!job) throw new MockError(404, `job ${id} not found`);
    if (!TERMINAL.has(job.view.state)) this.stopTimers(job);
    this.jobs = this.jobs.filter((j) => j !== job);
    this.onChange();
  }

  // -- internals -------------------------------------------------------------

  private mustFind(id: string, state?: string): MockJob {
    const job = this.jobs.find((j) => j.view.id === id);
    if (!job) throw new MockError(404, `job ${id} not found`);
    if (state && job.view.state !== state) {
      throw new MockError(400, `job ${id} is not in ${state} state`);
    }
    return job;
  }

  private start(job: MockJob): void {
    job.view.state = "running";
    job.runningSince = Date.now();
    const files = job.view.files ?? [];
    const first = files.find((f) => f.state === "pending");
    if (first) first.state = "running";
    this.onChange();
    const t = job.template;
    job.ticker = setInterval(() => this.tick(job), t.tickMs ?? 150);
    job.timers.push(job.ticker);
  }

  private tick(job: MockJob): void {
    const t = job.template;
    const files = job.view.files ?? [];
    const idx = files.findIndex((f) => f.state === "running");
    if (idx === -1) {
      this.finish(job);
      return;
    }
    const file = files[idx];
    if ((t.failFiles ?? []).includes(idx) && file.percent >= 0.7) {
      file.state = "failed";
      file.error = "mock conversion failed";
      this.advance(job);
      return;
    }
    file.percent = Math.min(1, file.percent + (t.progressPerTick ?? 0.1));
    file.speed = "1.8x";
    if (file.percent >= 1) {
      file.state = "done";
      this.advance(job);
      return;
    }
    this.updateProgress(job);
    this.onChange();
  }

  private advance(job: MockJob): void {
    const files = job.view.files ?? [];
    const next = files.find((f) => f.state === "pending");
    if (next) next.state = "running";
    this.updateProgress(job);
    if (!next) this.finish(job);
    else this.onChange();
  }

  private updateProgress(job: MockJob): void {
    const files = job.view.files ?? [];
    job.view.overall = files.length
      ? files.reduce((acc, f) => acc + f.percent, 0) / files.length
      : 0;
    job.view.done = files.filter((f) => f.state === "done").length;
    job.view.failed = files.filter((f) => f.state === "failed").length;
    if (job.runningSince) {
      const elapsed = Date.now() - job.runningSince;
      job.view.elapsed_ms = elapsed;
      job.view.eta_ms =
        job.view.overall > 0
          ? Math.round((elapsed / job.view.overall) * (1 - job.view.overall))
          : undefined;
    }
  }

  private finish(job: MockJob): void {
    this.stopTimers(job);
    const failed = job.view.failed;
    job.view.state = failed > 0 ? "failed" : "done";
    job.view.error = failed > 0 ? `${failed} of ${job.view.total} file(s) failed` : undefined;
    this.onChange();
  }

  private stopTimers(job: MockJob): void {
    for (const t of job.timers) clearTimeout(t);
    if (job.ticker) clearInterval(job.ticker);
    job.timers = [];
    job.ticker = undefined;
  }
}
