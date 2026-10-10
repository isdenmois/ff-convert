<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { createJob, store } from "./api";
import AnswersModal from "./components/AnswersModal.vue";
import JobCard from "./components/JobCard.vue";
import LibraryBrowser from "./components/LibraryBrowser.vue";

const error = ref("");
const answeringJobId = ref<string | null>(null);
// Jobs whose questions dialog the user explicitly closed; they are not
// auto-reopened until the job leaves the questions state.
const dismissed = new Set<string>();

const answeringJob = computed(() => store.jobs.find((j) => j.id === answeringJobId.value) ?? null);

// Auto-open the questions dialog for a job waiting on answers (initial load,
// page reload, or a dialog closed earlier). One dialog at a time: further
// pending jobs open via their "Answer questions" button, or here once the
// current one is answered.
watch(
  () => store.jobs,
  (jobs) => {
    for (const j of jobs) {
      if (j.state !== "questions") dismissed.delete(j.id);
    }
    if (answeringJob.value?.state !== "questions") {
      const next = jobs.find((j) => j.state === "questions" && !dismissed.has(j.id));
      if (next) answeringJobId.value = next.id;
    }
  },
  { immediate: true },
);

async function onConvert(path: string) {
  error.value = "";
  try {
    await createJob(path);
  } catch (e) {
    error.value = String(e);
  }
}

function openAnswers(id: string) {
  dismissed.delete(id);
  answeringJobId.value = id;
}

function closeAnswers() {
  if (answeringJobId.value) dismissed.add(answeringJobId.value);
  answeringJobId.value = null;
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
    @close="closeAnswers"
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
