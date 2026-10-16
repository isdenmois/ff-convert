import type { Locator, Page } from "@playwright/test";
import { PageElement } from "./element";

/** Page element for src/components/JobCard.vue. */
export class JobCardElement extends PageElement {
  get title(): Locator {
    return this.root.locator(".head strong");
  }

  get badge(): Locator {
    return this.root.locator(".badge");
  }

  get filesCounter(): Locator {
    return this.root.locator(".head > .muted");
  }

  get answerButton(): Locator {
    return this.root.getByRole("button", { name: "Answer questions" });
  }

  get cancelButton(): Locator {
    return this.root.getByRole("button", { name: "Cancel" });
  }

  get removeButton(): Locator {
    return this.root.getByRole("button", { name: "Remove" });
  }

  get overallPercent(): Locator {
    return this.root.locator(".overall > .pct");
  }

  get progressBar(): Locator {
    return this.root.locator(".overall .bar > div");
  }

  /** Elapsed/ETA line ("12s elapsed · eta 3s", "took 15s"). */
  get timeLabel(): Locator {
    return this.root.locator(".time");
  }

  /** The job's own output folder line (not the shared output root). */
  get outputFolder(): Locator {
    return this.root.locator("p.outdir");
  }

  fileRow(name: string): Locator {
    return this.root.locator(".file").filter({ hasText: name });
  }

  fileRowsByState(state: string): Locator {
    return this.root.locator(`.file[data-state=${state}]`);
  }
}

/** The queue section of the main page: job cards, newest first. */
export class JobQueueElement extends PageElement {
  constructor(page: Page) {
    super(page.getByRole("heading", { name: "Queue" }));
  }

  get emptyMessage(): Locator {
    return this.page.getByText("No jobs yet");
  }

  card(name: string): JobCardElement {
    return new JobCardElement(this.page.getByTestId("job-card").filter({ hasText: name }));
  }

  async cardCount(): Promise<number> {
    return this.page.getByTestId("job-card").count();
  }
}
