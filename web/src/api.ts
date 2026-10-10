import { reactive } from 'vue'
import type { Answers, JobView, LibraryEntry } from './types'

export const store = reactive({
  jobs: [] as JobView[],
  ws: 'connecting' as 'connecting' | 'open' | 'closed',
})

async function api<T>(method: string, url: string, body?: unknown): Promise<T> {
  const res = await fetch(url, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error((data as { error?: string }).error ?? res.statusText)
  }
  return data as T
}

export function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${proto}://${location.host}/api/ws`)
  ws.onopen = () => (store.ws = 'open')
  ws.onclose = () => {
    store.ws = 'closed'
    setTimeout(connectWS, 2000)
  }
  ws.onmessage = (ev) => {
    try {
      store.jobs = JSON.parse(ev.data) as JobView[]
    } catch {
      /* ignore malformed frames */
    }
  }
}

export function refreshJobs() {
  api<JobView[]>('GET', '/api/jobs')
    .then((jobs) => (store.jobs = jobs))
    .catch(() => {})
}

export function createJob(path: string): Promise<JobView> {
  return api<JobView>('POST', '/api/jobs', { path })
}

export function submitAnswers(id: string, answers: Answers[]): Promise<JobView> {
  return api<JobView>('POST', `/api/jobs/${id}/answers`, { answers })
}

export function cancelJob(id: string): Promise<JobView> {
  return api<JobView>('POST', `/api/jobs/${id}/cancel`)
}

export function removeJob(id: string): Promise<void> {
  return api<void>('DELETE', `/api/jobs/${id}`)
}

export function listLibrary(path: string): Promise<LibraryEntry[]> {
  return api('GET', `/api/library?path=${encodeURIComponent(path)}`)
}
