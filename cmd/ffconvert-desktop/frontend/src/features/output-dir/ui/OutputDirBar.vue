<script setup lang="ts">
import { onMounted } from "vue";
import { useOutputDirStore } from "../model/output-dir";

const store = useOutputDirStore();

onMounted(store.refresh);
</script>

<template>
  <span class="muted outdir truncate" :title="store.current">Output: {{ store.current }}</span>
  <button @click="store.change">Change…</button>
  <button
    v-if="store.current && store.current !== store.default"
    title="Back to the configured default output folder"
    @click="store.reset"
  >
    Reset
  </button>
</template>

<style scoped>
.outdir {
  max-width: 320px;
}
</style>
