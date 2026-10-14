// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from "@testing-library/vue";
import { afterEach, describe, expect, it } from "vitest";
import type { JobView } from "../api/job";
import JobCard from "./JobCard.vue";

afterEach(cleanup);

function baseJob(overrides: Partial<JobView> = {}): JobView {
  return {
    id: "j1",
    path: "/lib/movie",
    name: "movie",
    state: "running",
    created_at: "2026-01-01T00:00:00Z",
    overall: 0,
    done: 0,
    failed: 0,
    total: 0,
    ...overrides,
  };
}

describe("JobCard", () => {
  describe("rendering", () => {
    it("should show the job name and a human-readable badge when waiting on answers", () => {
      // Arrange
      const job = baseJob({ name: "movie", state: "questions" });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("movie")).toBeTruthy();
      expect(screen.getByText("needs answers")).toBeTruthy();
    });

    it("should fall back to the raw state text for an unknown state", () => {
      // Arrange
      const job = baseJob({ state: "weird" });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("weird")).toBeTruthy();
    });

    it("should show the done/total files count once files are known", () => {
      // Arrange
      const job = baseJob({ done: 2, total: 5 });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("2/5 files")).toBeTruthy();
    });

    it("should show the overall progress rounded to whole percent", () => {
      // Arrange
      const job = baseJob({ done: 2, total: 5, overall: 0.4235 });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("42%")).toBeTruthy();
    });

    it("should show elapsed time and eta while running", () => {
      // Arrange
      const job = baseJob({ total: 5, elapsed_ms: 5000, eta_ms: 10000 });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("0:05 elapsed · eta 0:10")).toBeTruthy();
    });

    it("should show the total time instead of eta once finished", () => {
      // Arrange
      const job = baseJob({ state: "done", total: 5, elapsed_ms: 5000 });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("took 0:05")).toBeTruthy();
    });

    it("should hide the time label when no time has elapsed", () => {
      // Arrange
      const job = baseJob({ total: 5 });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.queryByText(/elapsed|took/)).toBeNull();
    });

    it("should show the job error", () => {
      // Arrange
      const job = baseJob({ error: "probe failed" });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("probe failed")).toBeTruthy();
    });

    it("should show the output dir", () => {
      // Arrange
      const job = baseJob({ output_dir: "/out/movie" });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("/out/movie")).toBeTruthy();
    });

    it("should list skipped files by name", () => {
      // Arrange
      const job = baseJob({
        skipped: [
          { path: "/in/a.mkv", name: "a.mkv", output: "/out/a.mkv" },
          { path: "/in/b.mkv", name: "b.mkv", output: "/out/b.mkv" },
        ],
      });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("2 skipped")).toBeTruthy();
    });

    it("should show a done file", () => {
      // Arrange
      const job = baseJob({
        files: [
          { path: "/in/1.mkv", name: "1.mkv", output: "/out/1.mkv", state: "done", percent: 1 },
          { path: "/in/2.mkv", name: "2.mkv", output: "/out/2.mkv", state: "pending", percent: 0 },
        ],
      });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("1.mkv")).toBeTruthy();
      expect(screen.getByText("✓")).toBeTruthy();
    });

    it("should show progress and speed for a running file", () => {
      // Arrange
      const job = baseJob({
        total: 2,
        files: [
          {
            path: "/in/2.mkv",
            name: "2.mkv",
            output: "",
            state: "running",
            percent: 0.42,
            speed: "3x",
          },
          { path: "/in/3.mkv", name: "3.mkv", output: "", state: "pending", percent: 0 },
        ],
      });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("▶")).toBeTruthy();
      expect(screen.getByText("42%")).toBeTruthy();
    });

    it("should mark a failed file and expose its error", () => {
      // Arrange
      const job = baseJob({
        files: [
          {
            path: "/in/3.mkv",
            name: "3.mkv",
            output: "",
            state: "failed",
            percent: 0,
            error: "boom",
          },
          { path: "/in/4.mkv", name: "4.mkv", output: "", state: "pending", percent: 0 },
        ],
      });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.getByText("✗")).toBeTruthy();
      expect(screen.getByText("failed").getAttribute("title")).toBe("boom");
    });

    it("should hide counter, progress and file rows when no files are known yet", () => {
      // Arrange
      const job = baseJob();

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.queryByText(/files$/)).toBeNull();
      expect(screen.queryByText("0%")).toBeNull();
      expect(screen.queryByTestId("job-card")?.querySelector(".file")).toBeNull();
    });
  });

  describe("interactions", () => {
    it("should emit answer with the job id when Answer questions is clicked", async () => {
      // Arrange
      const job = baseJob({ state: "questions" });
      const view = render(JobCard, { props: { job } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Answer questions" }));

      // Assert
      expect(view.emitted("answer")).toEqual([["j1"]]);
    });

    it("should emit cancel with the job id when Cancel is clicked on a running job", async () => {
      // Arrange
      const job = baseJob({ state: "running" });
      const view = render(JobCard, { props: { job } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

      // Assert
      expect(view.emitted("cancel")).toEqual([["j1"]]);
    });

    it("should emit remove with the job id when Remove is clicked", async () => {
      // Arrange
      const job = baseJob({ state: "done" });
      const view = render(JobCard, { props: { job } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Remove" }));

      // Assert
      expect(view.emitted("remove")).toEqual([["j1"]]);
    });

    it("should hide the cancel and answer actions on a finished job", () => {
      // Arrange
      const job = baseJob({ state: "done" });

      // Act
      render(JobCard, { props: { job } });

      // Assert
      expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
      expect(screen.queryByRole("button", { name: "Answer questions" })).toBeNull();
      expect(screen.getByRole("button", { name: "Remove" })).toBeTruthy();
    });
  });
});
