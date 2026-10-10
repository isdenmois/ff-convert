<script setup lang="ts">
import { ref, computed } from 'vue'
import { store, createJob } from './api'
import LibraryBrowser from './components/LibraryBrowser.vue'
import JobCard from './components/JobCard.vue'
import AnswersModal from './components/AnswersModal.vue'

const error = ref('')
const answeringJobId = ref<string | null>(null)

const answeringJob = computed(() =>
  store.jobs.find((j) => j.id === answeringJobId.value) ?? null,
)

async function onConvert(path: string) {
  error.value = ''
  try {
    const job = await createJob(path)
    answeringJobId.value = job.id
  } catch (e) {
    error.value = String(e)
  }
}

function openAnswers(id: string) {
  answeringJobId.value = id
}
</script>

<template>
  <header>
    <h1>ff-convert</h1>
    <span class="muted">
      mkv transcoder · av1 / hevc / copy · opus stereo
      <template v-if="store.ws === 'open'"> · live</template>
      <template v-else> · offline</template>
    </span>
  </header>

  <p v-if="error" class="error">{{ error }}</p>

  <LibraryBrowser @convert="onConvert" />

  <h2>Queue</h2>
  <div v-if="store.jobs.length === 0" class="muted">No jobs yet — add a file or folder from the library above.</div>
  <JobCard
    v-for="job in [...store.jobs].reverse()"
    :key="job.id"
    :job="job"
    @answer="openAnswers"
  />

  <AnswersModal
    v-if="answeringJob?.questions && answeringJob.state === 'questions'"
    :job="answeringJob"
    @close="answeringJobId = null"
  />
</template>

<style scoped>
header {
  margin-bottom: 16px;
}
.error {
  color: var(--err);
}
</style>
