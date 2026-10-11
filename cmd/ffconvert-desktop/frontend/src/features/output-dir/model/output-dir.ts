import { defineStore } from "pinia";
import { ref } from "vue";
import { pickOutputFolder } from "@/shared/api/dialogs";
import { getOutputDirInfo, resetOutputDir, setOutputDir } from "@/shared/api/output-dir";

/** Output folder applied to newly added jobs (each job pins it at creation). */
export const useOutputDirStore = defineStore("output-dir", () => {
  const current = ref("");
  const defaultDir = ref("");
  const error = ref("");

  async function refresh() {
    try {
      const info = await getOutputDirInfo();
      current.value = info?.current ?? "";
      defaultDir.value = info?.default ?? "";
    } catch (e) {
      error.value = String(e);
    }
  }

  async function change() {
    error.value = "";
    try {
      const path = await pickOutputFolder();
      if (!path) return;
      await setOutputDir(path);
      await refresh();
    } catch (e) {
      error.value = String(e);
    }
  }

  async function reset() {
    error.value = "";
    try {
      await resetOutputDir();
      await refresh();
    } catch (e) {
      error.value = String(e);
    }
  }

  return { current, default: defaultDir, error, refresh, change, reset };
});
