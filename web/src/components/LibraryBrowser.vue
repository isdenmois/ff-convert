<script setup lang="ts">
import { onMounted, ref } from "vue";
import { listLibrary } from "../api";
import type { LibraryEntry } from "../types";

const emit = defineEmits<{ convert: [path: string] }>();

const entries = ref<LibraryEntry[]>([]);
const path = ref("");
const error = ref("");
const loading = ref(false);

async function load() {
  loading.value = true;
  error.value = "";
  try {
    entries.value = await listLibrary(path.value);
  } catch (e) {
    error.value = String(e);
  } finally {
    loading.value = false;
  }
}

function open(dir: LibraryEntry) {
  path.value = dir.path;
  load();
}

function up() {
  const parts = path.value.split("/").filter(Boolean);
  parts.pop();
  path.value = parts.join("/");
  load();
}

onMounted(load);
</script>

<template>
  <h2>Library</h2>
  <div class="toolbar">
    <button :disabled="path === ''" @click="up">↑ up</button>
    <span class="crumbs muted">
      / {{ path || '' }}
    </span>
    <button @click="load">refresh</button>
    <button v-if="path" class="primary" @click="emit('convert', path)">
      Convert this folder
    </button>
  </div>
  <p v-if="error" class="error">{{ error }}</p>
  <p v-else-if="loading" class="muted">loading…</p>
  <p v-else-if="entries.length === 0" class="muted">empty folder</p>
  <div v-else class="card list">
    <div v-for="e in entries" :key="e.path" class="row" :class="{ dir: e.dir }" @dblclick="e.dir && open(e)">
      <span class="icon">{{ e.dir ? '📁' : '🎬' }}</span>
      <span class="name" @click="e.dir && open(e)">{{ e.name }}</span>
      <button v-if="!e.dir" @click="emit('convert', e.path)">Convert</button>
    </div>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
.crumbs {
  flex: 1;
  font-family: monospace;
}
.list .row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 4px;
  border-bottom: 1px solid var(--border);
}
.list .row:last-child {
  border-bottom: none;
}
.row .name {
  flex: 1;
  cursor: default;
}
.row.dir .name {
  cursor: pointer;
  color: var(--text);
}
.row.dir:hover .name {
  color: var(--accent);
}
.error {
  color: var(--err);
}
</style>
