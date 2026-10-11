<script setup lang="ts">
import { useAddJobStore } from "../model/add-job";

const store = useAddJobStore();
</script>

<template>
  <button class="primary" @click="store.add('file')">Add file…</button>
  <button class="primary" @click="store.add('folder')">Add folder…</button>
</template>
