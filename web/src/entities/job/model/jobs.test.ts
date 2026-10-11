import type { JobView } from "@ffconvert/shared/api";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it } from "vitest";
import { nextTick } from "vue";
import { useJobsStore } from "./jobs";

function job(id: string, state: string): JobView {
  return { id, state } as JobView;
}

/** Pushes a full snapshot and lets the auto-open watcher flush. */
async function push(store: ReturnType<typeof useJobsStore>, list: JobView[]) {
  store.jobs = list;
  await nextTick();
}

describe("jobs store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("should auto-open the first job waiting on answers", async () => {
    // Arrange
    const store = useJobsStore();
    const snapshot = [job("a", "running"), job("b", "questions"), job("c", "questions")];

    // Act
    await push(store, snapshot);

    // Assert
    expect(store.answeringJob?.id).toBe("b");
  });

  it("should not reopen a dialog the user closed", async () => {
    // Arrange
    const store = useJobsStore();
    await push(store, [job("a", "questions")]);
    store.closeAnswers();

    // Act
    await push(store, [job("a", "questions")]);

    // Assert
    expect(store.answeringJob).toBeNull();
  });

  it("should reopen the dialog once the job leaves the questions state", async () => {
    // Arrange
    const store = useJobsStore();
    await push(store, [job("a", "questions")]);
    store.closeAnswers();
    await push(store, [job("a", "running")]);

    // Act
    await push(store, [job("a", "questions")]);

    // Assert
    expect(store.answeringJob?.id).toBe("a");
  });

  it("should move on to the next pending job once the current one is answered", async () => {
    // Arrange
    const store = useJobsStore();
    await push(store, [job("a", "questions"), job("b", "questions")]);

    // Act
    await push(store, [job("a", "running"), job("b", "questions")]);

    // Assert
    expect(store.answeringJob?.id).toBe("b");
  });

  it("should keep a job open across snapshots when it was reopened explicitly", async () => {
    // Arrange
    const store = useJobsStore();
    await push(store, [job("a", "questions")]);
    store.closeAnswers();
    store.openAnswers("a");

    // Act
    await push(store, [job("a", "questions")]);

    // Assert
    expect(store.answeringJob?.id).toBe("a");
  });

  it("should list newest jobs first", async () => {
    // Arrange
    const store = useJobsStore();
    const snapshot = [job("a", "running"), job("b", "running")];

    // Act
    await push(store, snapshot);

    // Assert
    expect(store.orderedJobs.map((j) => j.id)).toEqual(["b", "a"]);
  });
});
