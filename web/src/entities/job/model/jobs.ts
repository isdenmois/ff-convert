import type { Answers, JobView } from "@ffconvert/shared/api";
import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";
import { cancelJob, createJob, fetchJobs, removeJob, submitAnswers } from "@/shared/api/jobs";

/**
 * The live job queue: JobView snapshots pushed by the server plus the
 * questions-dialog bookkeeping shared by the queue and answers features.
 */
export const useJobsStore = defineStore("jobs", () => {
  const jobs = ref<JobView[]>([]);
  const ws = ref<"connecting" | "open" | "closed">("connecting");
  const answeringJobId = ref<string | null>(null);
  // Jobs whose questions dialog the user explicitly closed; they are not
  // auto-reopened until the job leaves the questions state.
  const dismissed = new Set<string>();

  /** Newest jobs first. */
  const orderedJobs = computed(() => [...jobs.value].reverse());
  const answeringJob = computed(
    () => jobs.value.find((j) => j.id === answeringJobId.value) ?? null,
  );

  // Auto-open the questions dialog for a job waiting on answers (initial load,
  // page reload, or a dialog closed earlier). One dialog at a time: further
  // pending jobs open via their "Answer questions" button, or here once the
  // current one is answered.
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
      /* state may have changed; WS will refresh */
    }
  }

  async function remove(id: string) {
    try {
      await removeJob(id);
    } catch {
      /* state may have changed; WS will refresh */
    }
  }

  function submit(id: string, answers: Answers[]) {
    return submitAnswers(id, answers);
  }

  /**
   * Starts the live jobs feed: one REST seed (covers the gap before the first
   * WS frame) plus the WS snapshot subscription with reconnect. The server
   * pushes a full JobView[] snapshot; mutations are not applied locally.
   */
  function init() {
    fetchJobs()
      .then((list) => (jobs.value = list))
      .catch(() => {});
    connectWS();
  }

  function connectWS() {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const socket = new WebSocket(`${proto}://${location.host}/api/ws`);
    socket.onopen = () => (ws.value = "open");
    socket.onclose = () => {
      ws.value = "closed";
      setTimeout(connectWS, 2000);
    };
    socket.onmessage = (ev) => {
      try {
        jobs.value = JSON.parse(ev.data) as JobView[];
      } catch {
        /* ignore malformed frames */
      }
    };
  }

  return {
    jobs,
    ws,
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
