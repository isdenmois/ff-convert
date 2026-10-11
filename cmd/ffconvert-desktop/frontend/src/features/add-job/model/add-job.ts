import { defineStore } from "pinia";
import { ref } from "vue";
import { useJobsStore } from "@/entities/job";
import { pickFile, pickFolder } from "@/shared/api/dialogs";

/** Adds jobs through the native file/folder pickers; owns the pick/add error. */
export const useAddJobStore = defineStore("add-job", () => {
  const error = ref("");

  async function add(kind: "file" | "folder") {
    error.value = "";
    try {
      const path = kind === "file" ? await pickFile() : await pickFolder();
      if (!path) return;
      await useJobsStore().addJob(path);
    } catch (e) {
      error.value = String(e);
    }
  }

  return { error, add };
});
