<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  AddJob,
  CancelJob,
  PickFile,
  PickFolder,
  RemoveJob,
  Snapshot,
  SubmitAnswers,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type { JobView } from './types'
import JobCard from './JobCard.vue'
import AnswersModal from './AnswersModal.vue'

const store = reactive({ jobs: [] as JobView[] })
const error = ref('')
const answeringJobId = ref<string | null>(null)

onMounted(async () => {
  try {
    store.jobs = (await Snapshot()) ?? []
  } catch (e) {
    error.value = String(e)
  }
  EventsOn('jobs', (jobs: JobView[]) => {
    store.jobs = jobs ?? []
  })
})

async function add(kind: 'file' | 'folder') {
  error.value = ''
  try {
    const path = kind === 'file' ? await PickFile() : await PickFolder()
    if (!path) return
    const job = await AddJob(path)
    if (job?.id) answeringJobId.value = job.id
  } catch (e) {
    error.value = String(e)
  }
}

function openAnswers(id: string) {
  answeringJobId.value = id
}

async function submit(id: string, answers: unknown) {
  await SubmitAnswers(id, answers as never)
}

async function cancel(id: string) {
  try {
    await CancelJob(id)
  } catch {
    /* state changed; events will refresh */
  }
}

async function remove(id: string) {
  try {
    await RemoveJob(id)
  } catch {
    /* state changed; events will refresh */
  }
}
</script>

<template>
  <header>
    <h1>ff-convert</h1>
    <span class="muted">mkv transcoder · av1 / hevc / copy · opus stereo</span>
  </header>

  <div class="toolbar">
    <button class="primary" @click="add('file')">Add file…</button>
    <button class="primary" @click="add('folder')">Add folder…</button>
  </div>

  <p v-if="error" class="error">{{ error }}</p>

  <h2>Queue</h2>
  <div v-if="store.jobs.length === 0" class="muted">
    No jobs yet — add a movie file or a folder with movie files.
  </div>
  <JobCard
    v-for="job in [...store.jobs].reverse()"
    :key="job.id"
    :job="job"
    @answer="openAnswers"
    @cancel="cancel"
    @remove="remove"
  />

  <AnswersModal
    v-if="store.jobs.find((j) => j.id === answeringJobId)?.state === 'questions'"
    :job="store.jobs.find((j) => j.id === answeringJobId)!"
    @close="answeringJobId = null"
    @submit="submit"
  />
</template>

<style scoped>
header {
  margin-bottom: 12px;
}
.toolbar {
  display: flex;
  gap: 8px;
  margin: 12px 0;
}
.error {
  color: var(--err);
}
</style>
