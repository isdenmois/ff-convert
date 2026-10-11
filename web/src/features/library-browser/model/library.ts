import { defineStore } from "pinia";
import { ref } from "vue";
import type { LibraryEntry } from "@/shared/api/library";
import { listLibrary } from "@/shared/api/library";

/** Server-side library navigation: current folder, its entries, load state. */
export const useLibraryStore = defineStore("library", () => {
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

  return { entries, path, error, loading, load, open, up };
});
