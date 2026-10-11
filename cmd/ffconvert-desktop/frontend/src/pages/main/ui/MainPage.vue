<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useJobsStore } from "@/entities/job";
import { AddJobButtons, useAddJobStore } from "@/features/add-job";
import { AnswersDialog } from "@/features/answers-dialog";
import { JobQueue } from "@/features/job-queue";
import { OutputDirBar, useOutputDirStore } from "@/features/output-dir";

const jobsStore = useJobsStore();
const addJobStore = useAddJobStore();
const outputDirStore = useOutputDirStore();
const initError = ref("");

// One error line in the same spot as before: whichever action failed last.
const error = computed(() => addJobStore.error || outputDirStore.error || initError.value);

onMounted(async () => {
  try {
    await jobsStore.init();
  } catch (e) {
    initError.value = String(e);
  }
});
</script>

<template>
  <header>
    <h1>ff-convert</h1>
    <span class="muted">mkv transcoder · av1 / hevc / copy · opus stereo</span>
  </header>

  <div class="toolbar">
    <AddJobButtons />
    <span class="spacer" />
    <OutputDirBar />
  </div>

  <p v-if="error" class="error">{{ error }}</p>

  <JobQueue />

  <AnswersDialog />
</template>

<style scoped>
header {
  margin-bottom: 12px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 12px 0;
}
.toolbar .spacer {
  flex: 1;
}
.error {
  color: var(--err);
}
</style>
