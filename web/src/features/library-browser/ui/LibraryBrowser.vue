<script setup lang="ts">
import { onMounted } from "vue";
import { useLibraryStore } from "../model/library";

const emit = defineEmits<{ convert: [path: string] }>();
const store = useLibraryStore();

onMounted(store.load);
</script>

<template>
  <h2>Library</h2>
  <div class="toolbar" data-testid="library-browser">
    <button :disabled="store.path === ''" @click="store.up">↑ up</button>
    <span class="crumbs muted">
      / {{ store.path || '' }}
    </span>
    <button @click="store.load">refresh</button>
    <button v-if="store.path" class="primary" @click="emit('convert', store.path)">
      Convert this folder
    </button>
  </div>
  <p v-if="store.error" class="error">{{ store.error }}</p>
  <p v-else-if="store.loading" class="muted">loading…</p>
  <p v-else-if="store.entries.length === 0" class="muted">empty folder</p>
  <div v-else class="card list">
    <div
      v-for="e in store.entries"
      :key="e.path"
      class="row"
      :class="{ dir: e.dir }"
      data-testid="library-row"
      :data-dir="e.dir"
      @dblclick="e.dir && store.open(e)"
    >
      <span class="icon">{{ e.dir ? '📁' : '🎬' }}</span>
      <span class="name" @click="e.dir && store.open(e)">{{ e.name }}</span>
      <button v-if="!e.dir" @click="emit('convert', e.path)">Convert</button>
    </div>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
.crumbs {
  flex: 1 1 auto;
  min-width: 0;
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: default;
}
.row.dir .name {
  cursor: pointer;
  color: var(--text);
}
.row.dir:hover .name {
  color: var(--accent);
}
</style>
