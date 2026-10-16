// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from "@testing-library/vue";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { GroupView, JobView, QuestionsView } from "../api/job";
import AnswersModal from "./AnswersModal.vue";

afterEach(cleanup);

function group(overrides: Partial<GroupView> = {}): GroupView {
  return {
    key: "g1",
    file_count: 2,
    files: ["a.mkv", "b.mkv"],
    tasks: [],
    audio: [
      { index: 0, label: "und (AAC 5.1)", language: "und" },
      { index: 1, label: "rus commentary (AAC)", language: "rus" },
    ],
    subs: [
      { index: 2, label: "eng (srt)", language: "eng" },
      { index: 3, label: "rus (ass)", language: "rus" },
    ],
    audio_defaults: [0],
    subs_defaults: [2],
    ...overrides,
  };
}

function questions(overrides: Partial<QuestionsView> = {}): QuestionsView {
  return {
    groups: [group()],
    preset_choices: ["copy", "h264"],
    ask_preset: true,
    ask_normalize: true,
    ask_attachments: true,
    default_preset: "copy",
    defaults: { audio: [], subs: [], preset: "copy", normalize: true, attachments: true },
    ...overrides,
  };
}

function questionsJob(overrides: Partial<JobView> = {}): JobView {
  return {
    id: "j1",
    path: "/lib/movie",
    name: "movie",
    state: "questions",
    created_at: "2026-01-01T00:00:00Z",
    overall: 0,
    done: 0,
    failed: 0,
    total: 1,
    output_dir: "/out/movie",
    questions: questions(),
    ...overrides,
  };
}

describe("AnswersModal", () => {
  describe("rendering", () => {
    it("should show the job name and output dir in the header", () => {
      // Arrange
      const job = questionsJob();

      // Act
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Assert
      expect(screen.getByText("movie")).toBeTruthy();
      expect(screen.getByText("/out/movie")).toBeTruthy();
    });

    it("should preselect the default audio and subtitle tracks", () => {
      // Arrange
      const job = questionsJob();

      // Act
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Assert
      expect((screen.getByLabelText("und (AAC 5.1)") as HTMLInputElement).checked).toBe(true);
      expect((screen.getByLabelText("rus commentary (AAC)") as HTMLInputElement).checked).toBe(
        false,
      );
      expect((screen.getByLabelText("eng (srt)") as HTMLInputElement).checked).toBe(true);
      expect((screen.getByLabelText("rus (ass)") as HTMLInputElement).checked).toBe(false);
    });

    it("should render nothing preselected when the backend sends null defaults", () => {
      // Arrange — older backends encode an empty default list as JSON null
      const legacy = questions({
        groups: [
          group({
            audio_defaults: null as unknown as number[],
            subs_defaults: null as unknown as number[],
          }),
        ],
      });
      const job = questionsJob({ questions: legacy });

      // Act
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Assert
      expect((screen.getByLabelText("und (AAC 5.1)") as HTMLInputElement).checked).toBe(false);
      expect((screen.getByLabelText("rus commentary (AAC)") as HTMLInputElement).checked).toBe(
        false,
      );
    });

    it("should note the shared layout instead of nav buttons for a single group", () => {
      // Arrange
      const job = questionsJob();

      // Act
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Assert
      expect(screen.queryByRole("button", { name: /Group 1/ })).toBeNull();
    });

    it("should switch the shown tracks when another group is selected", async () => {
      // Arrange
      const second = group({
        key: "g2",
        file_count: 1,
        files: ["c.mkv"],
        audio: [{ index: 0, label: "jpn (AAC)", language: "jpn" }],
        subs: [],
        audio_defaults: [0],
        subs_defaults: [],
      });
      const job = questionsJob({ questions: questions({ groups: [group(), second] }) });
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Group 2 (1 files)" }));

      // Assert
      expect(screen.getByLabelText("jpn (AAC)")).toBeTruthy();
      expect(screen.queryByLabelText("und (AAC 5.1)")).toBeNull();
    });

    it("should offer the preset choices with the default selected", () => {
      // Arrange
      const job = questionsJob();

      // Act
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Assert
      expect((screen.getByDisplayValue("copy") as HTMLSelectElement).value).toBe("copy");
      expect(screen.getByRole("option", { name: "h264" })).toBeTruthy();
    });

    it("should reflect the normalize and attachments defaults in their checkboxes", () => {
      // Arrange
      const job = questionsJob();

      // Act
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Assert
      expect((screen.getByLabelText("enabled") as HTMLInputElement).checked).toBe(true);
      expect((screen.getByLabelText("copy") as HTMLInputElement).checked).toBe(true);
    });

    it("should render no category select without a categories list", () => {
      // Arrange
      const job = questionsJob();

      // Act
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Assert
      expect(screen.queryByDisplayValue("— no category —")).toBeNull();
      expect(screen.queryByText(/Category/)).toBeNull();
    });
  });

  describe("submitting", () => {
    it("should submit the drafts with the queue-wide choices and close", async () => {
      // Arrange
      const submit = vi.fn().mockResolvedValue(undefined);
      const view = render(AnswersModal, { props: { job: questionsJob(), submit } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Start conversion" }));

      // Assert
      expect(submit).toHaveBeenCalledWith(
        "j1",
        [{ audio: [0], subs: [2], preset: "copy", normalize: true, attachments: true }],
        "",
      );
      expect(view.emitted("close")).toHaveLength(1);
    });

    it("should submit the toggled track selection", async () => {
      // Arrange
      const submit = vi.fn().mockResolvedValue(undefined);
      render(AnswersModal, { props: { job: questionsJob(), submit } });

      // Act
      await fireEvent.click(screen.getByLabelText("rus commentary (AAC)"));
      await fireEvent.click(screen.getByLabelText("eng (srt)"));
      await fireEvent.click(screen.getByLabelText("rus (ass)"));
      await fireEvent.click(screen.getByRole("button", { name: "Start conversion" }));

      // Assert
      expect(submit).toHaveBeenCalledWith(
        "j1",
        [{ audio: [0, 1], subs: [3], preset: "copy", normalize: true, attachments: true }],
        "",
      );
    });

    it("should submit the chosen output category and preview its folder", async () => {
      // Arrange
      const submit = vi.fn().mockResolvedValue(undefined);
      const job = questionsJob({ questions: questions({ categories: ["movies", "series"] }) });
      render(AnswersModal, { props: { job, submit } });

      // Act / Assert — "no category" selected by default, flat output shown.
      const select = screen.getByDisplayValue("— no category —") as HTMLSelectElement;
      expect(select.value).toBe("");
      expect(screen.getByText("/out/movie")).toBeTruthy();
      expect(screen.getByRole("option", { name: "series" })).toBeTruthy();

      // Act
      await fireEvent.update(select, "series");

      // Assert
      expect(screen.getByText("/out/movie/series")).toBeTruthy();

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Start conversion" }));

      // Assert
      expect(submit).toHaveBeenCalledWith(
        "j1",
        [{ audio: [0], subs: [2], preset: "copy", normalize: true, attachments: true }],
        "series",
      );
    });

    it("should preview a folder source's own subfolder inside the chosen category", async () => {
      // Arrange
      const job = questionsJob({
        output_dir: "/out",
        questions: questions({ categories: ["movies", "series"], source_folder: "Some Show" }),
      });
      render(AnswersModal, { props: { job, submit: vi.fn() } });

      // Assert — no category selected: the source folder sits at the root.
      expect(screen.getByText("/out/Some Show")).toBeTruthy();

      // Act
      await fireEvent.update(
        screen.getByDisplayValue("— no category —") as HTMLSelectElement,
        "series",
      );

      // Assert — the category folder comes first.
      expect(screen.getByText("/out/series/Some Show")).toBeTruthy();
    });

    it("should block submit and switch to a group that has no audio track selected", async () => {
      // Arrange
      const empty = group({
        key: "g2",
        file_count: 1,
        files: ["c.mkv"],
        audio: [{ index: 0, label: "jpn (AAC)", language: "jpn" }],
        subs: [],
        audio_defaults: [],
        subs_defaults: [],
      });
      const submit = vi.fn();
      render(AnswersModal, {
        props: {
          job: questionsJob({ questions: questions({ groups: [group(), empty] }) }),
          submit,
        },
      });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Start conversion" }));

      // Assert
      expect(screen.getByText("Select at least one audio track.")).toBeTruthy();
      expect(screen.getByLabelText("jpn (AAC)")).toBeTruthy();
      expect(submit).not.toHaveBeenCalled();
    });

    it("should surface the error and stay open when the submit rejects", async () => {
      // Arrange
      const submit = vi.fn().mockRejectedValue(new Error("nope"));
      const view = render(AnswersModal, { props: { job: questionsJob(), submit } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Start conversion" }));

      // Assert
      expect(screen.getByText("Error: nope")).toBeTruthy();
      expect(view.emitted("close")).toBeUndefined();
      expect(screen.getByRole("button", { name: "Start conversion" })).toBeTruthy();
    });

    it("should disable the submit button while submitting", async () => {
      // Arrange
      let resolveSubmit: (() => void) | undefined;
      const submit = vi.fn().mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            resolveSubmit = resolve;
          }),
      );
      render(AnswersModal, { props: { job: questionsJob(), submit } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Start conversion" }));

      // Assert
      const busy = screen.getByRole("button", { name: "Starting…" }) as HTMLButtonElement;
      expect(busy.disabled).toBe(true);

      resolveSubmit?.();
      expect(await screen.findByRole("button", { name: "Start conversion" })).toBeTruthy();
    });
  });

  describe("track guards", () => {
    it("should keep the last audio track checked when the user unchecks it", async () => {
      // Arrange
      render(AnswersModal, { props: { job: questionsJob(), submit: vi.fn() } });

      // Act
      await fireEvent.click(screen.getByLabelText("und (AAC 5.1)"));

      // Assert
      expect((screen.getByLabelText("und (AAC 5.1)") as HTMLInputElement).checked).toBe(true);
    });

    it("should allow unchecking every subtitle track", async () => {
      // Arrange
      const submit = vi.fn().mockResolvedValue(undefined);
      render(AnswersModal, { props: { job: questionsJob(), submit } });

      // Act
      await fireEvent.click(screen.getByLabelText("eng (srt)"));
      await fireEvent.click(screen.getByRole("button", { name: "Start conversion" }));

      // Assert
      expect(submit).toHaveBeenCalledWith("j1", [expect.objectContaining({ subs: [] })], "");
    });
  });

  describe("closing", () => {
    it("should emit close without submitting when Cancel is clicked", async () => {
      // Arrange
      const submit = vi.fn();
      const view = render(AnswersModal, { props: { job: questionsJob(), submit } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

      // Assert
      expect(view.emitted("close")).toHaveLength(1);
      expect(submit).not.toHaveBeenCalled();
    });

    it("should emit close when the × button is clicked", async () => {
      // Arrange
      const view = render(AnswersModal, { props: { job: questionsJob(), submit: vi.fn() } });

      // Act
      await fireEvent.click(screen.getByRole("button", { name: "×" }));

      // Assert
      expect(view.emitted("close")).toHaveLength(1);
    });

    it("should emit close when the backdrop is clicked", async () => {
      // Arrange
      const view = render(AnswersModal, { props: { job: questionsJob(), submit: vi.fn() } });

      // Act
      await fireEvent.click(screen.getByTestId("answers-modal"));

      // Assert
      expect(view.emitted("close")).toHaveLength(1);
    });
  });
});
