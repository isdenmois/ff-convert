import type { Answers, JobView } from "@ffconvert/shared/api";
import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";
import {
  cancelJob,
  createJob,
  removeJob,
  snapshotJobs,
  submitAnswers,
  subscribeJobs,
} from "@/shared/api/jobs";

/**
 * The live job queue: JobView snapshots pushed by the Go side plus the
 * questions-dialog bookkeeping shared by the queue and answers features.
 */
export const useJobsStore = defineStore("jobs", () => {
  const jobs = ref<JobView[]>([]);
  const answeringJobId = ref<string | null>(null);
  // Jobs whose questions dialog the user explicitly closed; they are not
  // auto-reopened until the job leaves the questions state.
  const dismissed = new Set<string>();

  /** Newest jobs first. */
  const orderedJobs = computed(() => [...jobs.value].reverse());
  const answeringJob = computed(
    () => jobs.value.find((j) => j.id === answeringJobId.value) ?? null,
  );

  // Auto-open the questions dialog for a job waiting on answers (app start,
  // or a dialog closed earlier). One dialog at a time: further pending jobs
  // open via their "Answer questions" button, or here once the current one is
  // answered.
  watch(
    jobs,
    (list) => {
      for (const j of list) {
        if (j.state !== "questions") dismissed.delete(j.id);
      }
      if (answeringJob.value?.state !== "questions") {
        const next = list.find((j) => j.state === "questions" && !dismissed.has(j.id));
        if (next) answeringJobId.value = next.id;
      }
    },
    { immediate: true },
  );

  function openAnswers(id: string) {
    dismissed.delete(id);
    answeringJobId.value = id;
  }

  function closeAnswers() {
    if (answeringJobId.value) dismissed.add(answeringJobId.value);
    answeringJobId.value = null;
  }

  /** Adds a job; errors propagate to the caller for display. */
  function addJob(path: string) {
    return createJob(path);
  }

  async function cancel(id: string) {
    try {
      await cancelJob(id);
    } catch {
      /* state changed; events will refresh */
    }
  }

  async function remove(id: string) {
    try {
      await removeJob(id);
    } catch {
      /* state changed; events will refresh */
    }
  }

  function submit(id: string, answers: Answers[]) {
    return submitAnswers(id, answers);
  }

  /**
   * Loads the queue once via an IPC snapshot and keeps it live by subscribing
   * to the Go-side "jobs" events (each frame carries the full snapshot).
   * Snapshot failures propagate to the caller for display.
   */
  async function init() {
    jobs.value = (await snapshotJobs()) ?? [];
    subscribeJobs((list) => (jobs.value = list));
  }

  return {
    jobs,
    orderedJobs,
    answeringJob,
    openAnswers,
    closeAnswers,
    addJob,
    cancel,
    remove,
    submit,
    init,
  };
});
