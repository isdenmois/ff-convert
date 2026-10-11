export interface StreamView {
  index: number;
  label: string;
  language: string;
  title?: string;
  codec?: string;
  channels?: number;
  layout?: string;
}

export interface TaskView {
  path: string;
  name: string;
}

export interface GroupView {
  key: string;
  file_count: number;
  files: string[];
  tasks: TaskView[];
  audio: StreamView[];
  subs: StreamView[];
  audio_defaults: number[];
  subs_defaults: number[];
}

export interface Answers {
  audio: number[];
  subs: number[];
  preset: string;
  normalize: boolean;
  attachments: boolean;
}

export interface QuestionsView {
  groups: GroupView[];
  preset_choices: string[];
  ask_preset: boolean;
  ask_normalize: boolean;
  ask_attachments: boolean;
  default_preset: string;
  /** Selectable output subfolders (web frontend only; absent elsewhere). */
  categories?: string[];
  defaults: Answers;
}

export interface FileView {
  path: string;
  name: string;
  output: string;
  state: string;
  percent: number;
  speed?: string;
  error?: string;
}

export interface SkippedView {
  path: string;
  name: string;
  output: string;
}

export interface JobView {
  id: string;
  path: string;
  name: string;
  /** Output folder pinned when the job was created (desktop only). */
  output_dir?: string;
  state: string;
  created_at: string;
  questions?: QuestionsView;
  files?: FileView[];
  skipped?: SkippedView[];
  overall: number;
  done: number;
  failed: number;
  total: number;
  /** Conversion wall time so far in ms (total run duration once finished). */
  elapsed_ms?: number;
  /** Estimated remaining conversion time in ms (running jobs only). */
  eta_ms?: number;
  error?: string;
}
