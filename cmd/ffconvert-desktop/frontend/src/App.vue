<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import {
  AddJob,
  CancelJob,
  PickFile,
  PickFolder,
  RemoveJob,
  Snapshot,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type { JobView } from './types'
import JobCard from './JobCard.vue'
import AnswersModal from './AnswersModal.vue'

const store = reactive({ jobs: [] as JobView[] })
const error = ref('')
const answeringJobId = ref<string | null>(null)
// Jobs whose questions dialog the user explicitly closed; they are not
// auto-reopened until the job leaves the questions state.
const dismissed = new Set<string>()

const answeringJob = computed(
  () => store.jobs.find((j) => j.id === answeringJobId.value) ?? null,
)

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

// Auto-open the questions dialog for a job waiting on answers (app start,
// or a dialog closed earlier). One dialog at a time: further pending jobs
// open via their "Answer questions" button, or here once the current one is
// answered.
watch(
  () => store.jobs,
  (jobs) => {
    for (const j of jobs) {
      if (j.state !== 'questions') dismissed.delete(j.id)
    }
    if (answeringJob.value?.state !== 'questions') {
      const next = jobs.find((j) => j.state === 'questions' && !dismissed.has(j.id))
      if (next) answeringJobId.value = next.id
    }
  },
  { immediate: true },
)

async function add(kind: 'file' | 'folder') {
  error.value = ''
  try {
    const path = kind === 'file' ? await PickFile() : await PickFolder()
    if (!path) return
    await AddJob(path)
  } catch (e) {
    error.value = String(e)
  }
}

function openAnswers(id: string) {
  dismissed.delete(id)
  answeringJobId.value = id
}

function closeAnswers() {
  if (answeringJobId.value) dismissed.add(answeringJobId.value)
  answeringJobId.value = null
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
    v-if="answeringJob?.questions && answeringJob.state === 'questions'"
    :job="answeringJob"
    @close="closeAnswers"
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
