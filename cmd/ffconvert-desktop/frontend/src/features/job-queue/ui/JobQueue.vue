<script setup lang="ts">
import { JobCard } from "@ffconvert/shared/ui";
import { useJobsStore } from "@/entities/job";

const store = useJobsStore();
</script>

<template>
  <h2>Queue</h2>
  <div v-if="store.jobs.length === 0" class="muted">
    No jobs yet.
  </div>
  <JobCard
    v-for="job in store.orderedJobs"
    :key="job.id"
    :job="job"
    @answer="store.openAnswers"
    @cancel="store.cancel"
    @remove="store.remove"
  />
</template>
