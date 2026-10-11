<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import type { Answers, JobView } from "../api/job";

const props = defineProps<{
  job: JobView;
  /** Submits the answers for the job; the modal stays open until it resolves. */
  submit: (id: string, answers: Answers[], category: string) => Promise<unknown>;
}>();
const emit = defineEmits<{ close: [] }>();

// The modal is only rendered when the job has pending questions.
// biome-ignore lint/style/noNonNullAssertion: guaranteed by the parent (the page only opens this when questions exist)
const q = props.job.questions!;
const groupIdx = ref(0);
const group = computed(() => q.groups[groupIdx.value]);

// Per-group draft answers, initialized from defaults. The ?? [] guards
// against older backends encoding an empty default list as JSON null
// (spreading null throws). The video preset, normalization and attachments
// are not part of the drafts — they are one set of choices for the whole
// queue item.
const drafts = reactive<Omit<Answers, "preset" | "normalize" | "attachments">[]>(
  q.groups.map((g) => ({
    audio: [...(g.audio_defaults ?? [])],
    subs: [...(g.subs_defaults ?? [])],
  })),
);
// Queue-item-wide choices, preselected with the defaults ("copy" preset,
// normalization on, attachments copy — unless the config overrides);
// replicated into every group draft on submit.
const preset = ref(q.default_preset);
const normalize = ref(q.defaults.normalize);
const attachments = ref(q.defaults.attachments);
// Output subfolder for the whole queue item ("" = the output root). Only
// offered when the backend sends a categories list (web frontend).
const category = ref("");
const outputDir = computed(() =>
  category.value ? `${props.job.output_dir}/${category.value}` : props.job.output_dir,
);
const draft = computed(() => drafts[groupIdx.value]);

const error = ref("");
const busy = ref(false);

function toggle(list: "audio" | "subs", index: number, e: Event) {
  const arr = draft.value[list];
  const i = arr.indexOf(index);
  if (i >= 0) {
    if (list === "audio" && arr.length === 1) {
      // keep ≥1 audio track; re-sync the checkbox the browser just unchecked
      (e.target as HTMLInputElement).checked = true;
      return;
    }
    arr.splice(i, 1);
  } else {
    arr.push(index);
  }
}

async function onSubmit() {
  for (const [i, d] of drafts.entries()) {
    if (d.audio.length === 0) {
      groupIdx.value = i;
      error.value = "Select at least one audio track.";
      return;
    }
  }
  busy.value = true;
  error.value = "";
  try {
    await props.submit(
      props.job.id,
      drafts.map((d) => ({
        ...d,
        preset: preset.value,
        normalize: normalize.value,
        attachments: attachments.value,
        audio: [...d.audio],
        subs: [...d.subs],
      })),
      category.value,
    );
    emit("close");
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="backdrop" data-testid="answers-modal" @click.self="emit('close')">
    <div class="modal">
      <header>
        <strong>{{ job.name }}</strong>
        <button class="close" @click="emit('close')">×</button>
      </header>
      <p v-if="job.output_dir" class="muted small outdir" :title="outputDir">
        Output: {{ outputDir }}
      </p>

      <section v-if="q.categories?.length" class="inline">
        <h3>Category — output subfolder (all files)</h3>
        <select v-model="category">
          <option value="">— no category —</option>
          <option v-for="c in q.categories" :key="c" :value="c">{{ c }}</option>
        </select>
      </section>

      <section v-if="q.ask_preset" class="inline">
        <h3>Video preset (all files)</h3>
        <select v-model="preset">
          <option v-for="p in q.preset_choices" :key="p" :value="p">
            {{ p }}
          </option>
        </select>
      </section>

      <section v-if="q.ask_normalize" class="inline">
        <h3>Normalize loudness — loudnorm i=-13 (all files)</h3>
        <label class="track">
          <input type="checkbox" v-model="normalize" />
          {{ normalize ? "enabled" : "disabled" }}
        </label>
      </section>

      <section v-if="q.ask_attachments" class="inline">
        <h3>Attachments — fonts, posters (all files)</h3>
        <label class="track">
          <input type="checkbox" v-model="attachments" />
          {{ attachments ? "copy" : "omit" }}
        </label>
      </section>

      <nav v-if="q.groups.length > 1">
        <button
          v-for="(g, i) in q.groups"
          :key="g.key"
          :class="{ active: i === groupIdx }"
          @click="groupIdx = i"
        >
          Group {{ i + 1 }} ({{ g.file_count }} files)
        </button>
      </nav>
      <p v-else class="muted">
        {{ group.file_count }} file(s) share this track layout.
      </p>

      <section>
        <h3>Audio tracks → opus stereo</h3>
        <label v-for="s in group.audio" :key="s.index" class="track">
          <input
            type="checkbox"
            :checked="draft.audio.includes(s.index)"
            @change="toggle('audio', s.index, $event)"
          />
          {{ s.label }}
        </label>
      </section>

      <section>
        <h3>Subtitles (copied)</h3>
        <p v-if="group.subs.length === 0" class="muted">none</p>
        <label v-for="s in group.subs" :key="s.index" class="track">
          <input
            type="checkbox"
            :checked="draft.subs.includes(s.index)"
            @change="toggle('subs', s.index, $event)"
          />
          {{ s.label }}
        </label>
      </section>

      <footer>
        <p v-if="error" class="error">{{ error }}</p>
        <span class="spacer" />
        <button @click="emit('close')">Cancel</button>
        <button class="primary" :disabled="busy" @click="onSubmit">
          {{ busy ? "Starting…" : "Start conversion" }}
        </button>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 40px 16px;
  overflow-y: auto;
  z-index: 10;
}
.modal {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  width: 640px;
  max-width: 100%;
  padding: 16px 20px;
}
header {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}
header strong {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.outdir {
  margin: 0 0 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
nav {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
nav button.active {
  border-color: var(--accent);
  color: var(--accent);
}
section {
  margin: 12px 0;
}
section.inline {
  display: flex;
  align-items: center;
  gap: 12px;
}
section.inline h3 {
  margin: 0;
  min-width: 240px;
}
h3 {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted);
  margin: 0 0 6px;
}
.track {
  display: block;
  padding: 3px 0;
  cursor: pointer;
}
.track input {
  margin-right: 8px;
}
footer {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}
.spacer {
  flex: 1;
}
.error {
  color: var(--err);
}
</style>
