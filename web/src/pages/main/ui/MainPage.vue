<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useJobsStore } from "@/entities/job";
import { AnswersDialog } from "@/features/answers-dialog";
import { JobQueue } from "@/features/job-queue";
import { LibraryBrowser } from "@/features/library-browser";

const store = useJobsStore();
const error = ref("");

onMounted(store.init);

async function onConvert(path: string) {
  error.value = "";
  try {
    await store.addJob(path);
  } catch (e) {
    error.value = String(e);
  }
}
</script>

<template>
  <header>
    <h1>ff-convert</h1>
    <span class="muted" data-testid="conn-status">
      mkv transcoder · av1 / hevc / copy · opus stereo
      <template v-if="store.ws === 'open'"> · live</template>
      <template v-else> · offline</template>
    </span>
  </header>

  <p v-if="error" class="error">{{ error }}</p>

  <LibraryBrowser @convert="onConvert" />

  <JobQueue />

  <AnswersDialog />
</template>

<style scoped>
header {
  margin-bottom: 16px;
}
.error {
  color: var(--err);
}
</style>
