import type { JobView } from "@ffconvert/shared/api";
import {
  AddJob,
  CancelJob,
  RemoveJob,
  Snapshot,
  SubmitAnswers,
} from "../../../wailsjs/go/main/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";

// Wails IPC bindings for the job queue. The generated bindings return
// class-based wails models, which are structurally identical to the shared
// DTO types (@ffconvert/shared/api).

export const createJob = AddJob;
export const submitAnswers = SubmitAnswers;
export const cancelJob = CancelJob;
export const removeJob = RemoveJob;
export const snapshotJobs = Snapshot;

/** Subscribes to the Go-side "jobs" event; each frame carries the full snapshot. */
export function subscribeJobs(onSnapshot: (jobs: JobView[]) => void): void {
  EventsOn("jobs", (jobs: JobView[]) => onSnapshot(jobs ?? []));
}
