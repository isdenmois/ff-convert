import type { Answers, JobView } from "@ffconvert/shared/api";
import { api } from "./client";

export function fetchJobs(): Promise<JobView[]> {
  return api<JobView[]>("GET", "/api/jobs");
}

export function createJob(path: string): Promise<JobView> {
  return api<JobView>("POST", "/api/jobs", { path });
}

export function submitAnswers(id: string, answers: Answers[]): Promise<JobView> {
  return api<JobView>("POST", `/api/jobs/${id}/answers`, { answers });
}

export function cancelJob(id: string): Promise<JobView> {
  return api<JobView>("POST", `/api/jobs/${id}/cancel`);
}

export function removeJob(id: string): Promise<void> {
  return api<void>("DELETE", `/api/jobs/${id}`);
}
