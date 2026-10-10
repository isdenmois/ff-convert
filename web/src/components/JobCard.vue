<script setup lang="ts">
import { computed } from "vue";
import { cancelJob, removeJob } from "../api";
import { formatDuration } from "../format";
import type { JobView } from "../types";

const props = defineProps<{ job: JobView }>();
const emit = defineEmits<{ answer: [id: string] }>();

const overallPct = computed(() => Math.round(props.job.overall * 100));
const timeLabel = computed(() => {
  const elapsed = props.job.elapsed_ms;
  if (!elapsed) return "";
  if (props.job.state === "running") {
    const eta = props.job.eta_ms;
    return eta
      ? `${formatDuration(elapsed)} elapsed · eta ${formatDuration(eta)}`
      : `${formatDuration(elapsed)} elapsed`;
  }
  return `took ${formatDuration(elapsed)}`;
});

const stateLabel: Record<string, string> = {
  probing: "probing",
  questions: "needs answers",
  queued: "queued",
  running: "running",
  done: "done",
  failed: "failed",
  canceled: "canceled",
};

async function onCancel() {
  try {
    await cancelJob(props.job.id);
  } catch {
    /* state may have changed; WS will refresh */
  }
}

async function onRemove() {
  try {
    await removeJob(props.job.id);
  } catch {
    /* state may have changed; WS will refresh */
  }
}
</script>

<template>
  <div class="card">
    <div class="head">
      <strong>{{ job.name }}</strong>
      <span class="badge" :class="job.state">{{ stateLabel[job.state] ?? job.state }}</span>
      <span v-if="job.total > 0" class="muted">{{ job.done }}/{{ job.total }} files</span>
      <span class="spacer" />
      <button v-if="job.state === 'questions'" class="primary" @click="emit('answer', job.id)">
        Answer questions
      </button>
      <button
        v-if="['probing', 'questions', 'queued', 'running'].includes(job.state)"
        @click="onCancel"
      >
        Cancel
      </button>
      <button @click="onRemove">Remove</button>
    </div>

    <p v-if="job.error" class="error">{{ job.error }}</p>

    <div v-if="job.total > 0" class="overall">
      <div class="bar"><div :style="{ width: overallPct + '%' }" /></div>
      <span class="muted pct">{{ overallPct }}%</span>
      <span v-if="timeLabel" class="muted small time">{{ timeLabel }}</span>
    </div>

    <div v-if="job.skipped?.length" class="muted small">
      skipped (already converted): {{ job.skipped.map((s) => s.name).join(', ') }}
    </div>

    <div v-if="job.files?.length" class="files">
      <div v-for="f in job.files" :key="f.path" class="file" :data-state="f.state">
        <span class="fstate">
          {{ f.state === 'done' ? '✓' : f.state === 'failed' ? '✗' : f.state === 'running' ? '▶' : '·' }}
        </span>
        <span class="fname">{{ f.name }}</span>
        <template v-if="f.state === 'running'">
          <div class="bar"><div :style="{ width: Math.round(f.percent * 100) + '%' }" /></div>
          <span class="muted small pct">{{ Math.round(f.percent * 100) }}% {{ f.speed }}</span>
        </template>
        <span v-else-if="f.state === 'failed'" class="error small" :title="f.error">failed</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.spacer {
  flex: 1;
}
.overall {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 6px 0;
}
.files {
  margin-top: 8px;
}
.file {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 0;
  font-size: 13px;
}
.file[data-state='done'] .fname { color: var(--ok); }
.file[data-state='failed'] .fname { color: var(--err); }
.file[data-state='pending'] .fname, .file[data-state='canceled'] .fname { color: var(--muted); }
.fname {
  min-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fstate {
  width: 14px;
  text-align: center;
}
.pct {
  min-width: 90px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.time {
  min-width: 140px;
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.small {
  font-size: 12px;
}
.error {
  color: var(--err);
}
</style>
