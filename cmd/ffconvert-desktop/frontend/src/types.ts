export interface StreamView {
  index: number
  label: string
  language: string
  title?: string
  codec?: string
  channels?: number
  layout?: string
}

export interface TaskView {
  path: string
  name: string
}

export interface GroupView {
  key: string
  file_count: number
  files: string[]
  tasks: TaskView[]
  audio: StreamView[]
  subs: StreamView[]
  audio_defaults: number[]
  subs_defaults: number[]
}

export interface Answers {
  audio: number[]
  subs: number[]
  preset: string
  normalize: boolean
  attachments: boolean
}

export interface QuestionsView {
  groups: GroupView[]
  preset_choices: string[]
  ask_preset: boolean
  ask_normalize: boolean
  ask_attachments: boolean
  default_preset: string
  defaults: Answers
}

export interface FileView {
  path: string
  name: string
  output: string
  state: string
  percent: number
  speed?: string
  error?: string
}

export interface SkippedView {
  path: string
  name: string
  output: string
}

export interface JobView {
  id: string
  path: string
  name: string
  state: string
  created_at: string
  questions?: QuestionsView
  files?: FileView[]
  skipped?: SkippedView[]
  overall: number
  done: number
  failed: number
  total: number
  error?: string
}

export interface LibraryEntry {
  name: string
  path: string
  dir: boolean
}
