import { expect, test } from "./fixtures/mock-client";

test.describe("conversion happy path", () => {
  test("convert a file → answer questions → progress → done", async ({ mainPage, mock }) => {
    await mock.seedLibrary({ Movies: { "movie1.mkv": true } });
    await mainPage.open();
    const lib = mainPage.library;

    await lib.openDir("Movies");
    await lib.convertFile("movie1.mkv");
    const card = mainPage.jobs.card("movie1.mkv");
    await expect(card.title).toHaveText("movie1.mkv");

    // probing parks in `questions`; the modal auto-opens for the pending job
    const modal = mainPage.modal;
    await expect(card.badge).toContainText("needs answers");
    await expect(modal.root).toBeVisible();
    await expect(modal.title).toContainText("movie1.mkv");
    await expect(modal.root).toContainText("1 file(s) share this track layout.");

    // defaults are preselected: first audio track on, the rest off
    await expect(modal.audioTrack("eng  AC3  5.1")).toBeChecked();
    await expect(modal.audioTrack("rus  AC3  5.1")).not.toBeChecked();
    await modal.screenshot("modal-open");

    await modal.submit();
    await expect(modal.root).toBeHidden();

    // running: overall bar, per-file percent + speed, elapsed label
    await expect(card.badge).toContainText("running");
    await expect(card.fileRow("movie1.mkv")).toContainText("%");
    await expect(card.fileRow("movie1.mkv")).toContainText("1.8x");
    await expect(card.timeLabel).toContainText("elapsed");
    await expect(card.overallPercent).toContainText("%");
    await card.screenshot("job-running");

    // done: checkmark glyphs, counters, total run time
    await expect(card.badge).toContainText("done", { timeout: 15_000 });
    await expect(card.filesCounter).toHaveText("1/1 files");
    await expect(card.fileRowsByState("done")).toHaveCount(1);
    await expect(card.fileRow("movie1.mkv")).toContainText("✓");
    await expect(card.timeLabel).toContainText("took");
    await card.screenshot("job-done");

    // the SPA submitted a well-formed answers payload
    const answers = (await mock.requests()).find((r) => r.path === "/api/jobs/1/answers");
    expect(answers?.body).toEqual({
      answers: [{ audio: [1], subs: [], preset: "copy", normalize: true, attachments: true }],
    });
  });
});
