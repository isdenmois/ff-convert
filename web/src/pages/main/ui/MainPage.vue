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
</style>
