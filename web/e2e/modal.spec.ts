import { expect, test } from "./fixtures/mock-client";

test.describe("answers modal", () => {
  test("single group: no tabs, shared-layout hint, track lists", async ({ mainPage, mock }) => {
    await mock.seedJob({ path: "Movies/movie.mkv" }, "questions");
    await mainPage.open();
    const modal = mainPage.modal;

    await expect(modal.root).toBeVisible();
    await expect(modal.title).toContainText("movie.mkv");
    await expect(modal.groupTabs).toHaveCount(0);
    await expect(modal.audioTracks).toHaveCount(2);
    await expect(modal.subTracks).toHaveCount(1);
    await expect(modal.subTrack("eng  SRT")).not.toBeChecked();
    await modal.screenshot("modal-single-group");
  });

  test("multi-group: tabs switch track lists", async ({ mainPage, mock }) => {
    await mock.seedJob(
      {
        path: "Movies",
        groups: [
          {
            files: ["a.mkv", "b.mkv"],
            audio: [
              { language: "eng", channels: 6 },
              { language: "rus", channels: 6 },
            ],
          },
          { files: ["c.mkv"], audio: [{ language: "jpn", channels: 2 }], subs: [] },
        ],
      },
      "questions",
    );
    await mainPage.open();
    const modal = mainPage.modal;

    await expect(modal.groupTab(1)).toHaveText("Group 1 (2 files)");
    await expect(modal.groupTab(2)).toHaveText("Group 2 (1 files)");
    await expect(modal.audioTrack("eng  AC3  5.1")).toBeVisible();

    await modal.groupTab(2).click();
    await expect(modal.audioTrack("jpn  AC3  stereo")).toBeVisible();
    await expect(modal.audioTrack("eng  AC3  5.1")).toBeHidden();
    await modal.screenshot("modal-group-2");
  });

  test("the last checked audio track cannot be deselected", async ({ mainPage, mock }) => {
    await mock.seedJob({ path: "Movies/m.mkv" }, "questions");
    await mainPage.open();
    const modal = mainPage.modal;
    const eng = modal.audioTrack("eng  AC3  5.1");

    await expect(eng).toBeChecked();
    await eng.click();
    await expect(eng).toBeChecked();
  });

  test("submitting with an audio-less group jumps to it and shows the error", async ({
    mainPage,
    mock,
  }) => {
    await mock.seedJob(
      {
        path: "Movies",
        groups: [{ files: ["a.mkv"] }, { files: ["b.mkv"], audio: [] }],
      },
      "questions",
    );
    await mainPage.open();
    const modal = mainPage.modal;

    await expect(modal.groupTab(1)).toHaveClass(/active/);
    await modal.submit();

    await expect(modal.errorText).toHaveText("Select at least one audio track.");
    await expect(modal.groupTab(2)).toHaveClass(/active/);
    await expect(modal.root).toBeVisible();
    await modal.screenshot("modal-validation-error");
  });

  test("preset, normalize and attachments controls", async ({ mainPage, mock }) => {
    await mock.seedJob({ path: "Movies/m.mkv" }, "questions");
    await mainPage.open();
    const modal = mainPage.modal;

    await expect(modal.presetSelect).toHaveValue("copy");
    await modal.presetSelect.selectOption("av1");
    await expect(modal.presetSelect).toHaveValue("av1");

    await expect(modal.normalizeLabel).toContainText("enabled");
    await modal.normalizeCheckbox.click();
    await expect(modal.normalizeLabel).toContainText("disabled");

    await expect(modal.attachmentsLabel).toContainText("copy");
    await modal.attachmentsCheckbox.click();
    await expect(modal.attachmentsLabel).toContainText("omit");
    await modal.screenshot("modal-options");
  });

  test("busy state while answers are being accepted", async ({ mainPage, mock }) => {
    await mock.seedJob({ path: "Movies/m.mkv", answerDelayMs: 700, startMs: 5000 }, "questions");
    await mainPage.open();
    const modal = mainPage.modal;

    await modal.submit();
    await expect(modal.busyButton).toBeVisible();
    await expect(modal.submitButton).toBeHidden();

    // once accepted the modal closes and the job moves on (still queued: startMs is high)
    await expect(modal.root).toBeHidden();
    await expect(mainPage.jobs.card("m.mkv").badge).toContainText("queued");
  });

  test("a rejected submission surfaces the error and keeps the modal open", async ({
    mainPage,
    mock,
  }) => {
    await mock.seedJob({ path: "Movies/m.mkv", answersError: "no such preset" }, "questions");
    await mainPage.open();
    const modal = mainPage.modal;

    await modal.submit();
    await expect(modal.errorText).toContainText("no such preset");
    await expect(modal.root).toBeVisible();
    await expect(modal.submitButton).toBeVisible();
  });

  test("category select chooses the output subfolder", async ({ mainPage, mock }) => {
    await mock.seedJob({ path: "Movies/movie.mkv", categories: ["movies", "series"] }, "questions");
    await mainPage.open();
    const modal = mainPage.modal;

    await expect(modal.categorySelect).toBeVisible();
    await expect(modal.categorySelect).toHaveValue("");
    await expect(modal.outputLine).not.toContainText("/series");

    await modal.categorySelect.selectOption("series");
    await expect(modal.outputLine).toContainText("/series");

    await modal.submit();
    await expect(modal.root).toBeHidden();

    const answers = (await mock.requests()).find((r) => r.path === "/api/jobs/1/answers");
    expect(answers?.body).toEqual({
      category: "series",
      answers: [{ audio: [1], subs: [], preset: "copy", normalize: true, attachments: true }],
    });
  });

  test("folder job keeps its folder inside the chosen category", async ({ mainPage, mock }) => {
    await mock.seedJob(
      {
        path: "Movies/Some Show",
        groups: [{ files: ["S01E01.mkv", "S01E02.mkv"] }],
        categories: ["movies", "series"],
      },
      "questions",
    );
    await mainPage.open();
    const modal = mainPage.modal;

    // The card shows the source folder before any category is chosen.
    await expect(mainPage.jobs.card("Some Show").outputFolder).toHaveText("converted/Some Show");
    await expect(modal.outputLine).toContainText("converted/Some Show");
    await modal.categorySelect.selectOption("series");
    await expect(modal.outputLine).toContainText("converted/series/Some Show");

    await modal.submit();
    await expect(modal.root).toBeHidden();
    await expect(mainPage.jobs.card("Some Show").outputFolder).toHaveText(
      "converted/series/Some Show",
    );
  });

  test("backdrop click closes the modal; the card reopens it", async ({ mainPage, mock }) => {
    await mock.seedJob({ path: "Movies/m.mkv" }, "questions");
    await mainPage.open();
    const modal = mainPage.modal;

    await expect(modal.root).toBeVisible();
    await modal.closeViaBackdrop();
    await expect(modal.root).toBeHidden();

    const card = mainPage.jobs.card("m.mkv");
    await card.answerButton.click();
    await expect(modal.root).toBeVisible();
    await expect(modal.audioTrack("eng  AC3  5.1")).toBeChecked();
  });
});
