<script setup lang="ts">
import { AnswersModal } from "@ffconvert/shared/ui";
import { computed } from "vue";
import { useJobsStore } from "@/entities/job";

const store = useJobsStore();
const answeringJob = computed(() => store.answeringJob);
</script>

<template>
  <AnswersModal
    v-if="answeringJob?.questions && answeringJob.state === 'questions'"
    :job="answeringJob"
    :submit="store.submit"
    @close="store.closeAnswers"
  />
</template>
