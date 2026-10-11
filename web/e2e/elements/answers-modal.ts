import type { Locator, Page } from "@playwright/test";
import { PageElement } from "./element";

/** Page element for src/components/AnswersModal.vue. */
export class AnswersModalElement extends PageElement {
  constructor(page: Page) {
    super(page.getByTestId("answers-modal"));
  }

  get title(): Locator {
    return this.root.locator("header strong");
  }

  get presetSelect(): Locator {
    return this.root.locator("section", { hasText: "Video preset" }).locator("select");
  }

  get categorySelect(): Locator {
    return this.section("Category").locator("select");
  }

  get outputLine(): Locator {
    return this.root.locator("p.outdir");
  }

  get normalizeCheckbox(): Locator {
    return this.section("Normalize loudness").locator("input[type=checkbox]");
  }

  get normalizeLabel(): Locator {
    return this.section("Normalize loudness").locator(".track");
  }

  get attachmentsCheckbox(): Locator {
    return this.section("Attachments").locator("input[type=checkbox]");
  }

  get attachmentsLabel(): Locator {
    return this.section("Attachments").locator(".track");
  }

  get groupTabs(): Locator {
    return this.root.locator("nav button");
  }

  groupTab(index1: number): Locator {
    return this.groupTabs.nth(index1 - 1);
  }

  get audioTracks(): Locator {
    return this.section("Audio tracks").locator("label.track");
  }

  get subTracks(): Locator {
    return this.section("Subtitles").locator("label.track");
  }

  audioTrack(label: string): Locator {
    return this.section("Audio tracks").getByLabel(label);
  }

  subTrack(label: string): Locator {
    return this.section("Subtitles").getByLabel(label);
  }

  get submitButton(): Locator {
    return this.root.getByRole("button", { name: "Start conversion" });
  }

  get busyButton(): Locator {
    return this.root.getByRole("button", { name: "Starting…" });
  }

  get cancelButton(): Locator {
    return this.root.getByRole("button", { name: "Cancel", exact: true });
  }

  get errorText(): Locator {
    return this.root.locator("footer .error");
  }

  async submit(): Promise<void> {
    await this.submitButton.click();
  }

  async close(): Promise<void> {
    await this.cancelButton.click();
  }

  /** Click the backdrop itself (top-left corner) to close, like a stray click. */
  async closeViaBackdrop(): Promise<void> {
    await this.root.click({ position: { x: 5, y: 5 } });
  }

  private section(heading: string): Locator {
    return this.root.locator("section", { hasText: heading });
  }
}
