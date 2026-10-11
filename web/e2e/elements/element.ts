import { type Locator, test } from "@playwright/test";

/**
 * Base of the Page Element pattern: wraps a single Playwright Locator and
 * adds small, intention-revealing helpers. Concrete elements (JobCardElement,
 * AnswersModalElement, …) layer domain-specific getters and actions on top.
 */
export class PageElement {
  constructor(readonly root: Locator) {}

  /** The page the element lives on. */
  get page() {
    return this.root.page();
  }

  async isVisible(): Promise<boolean> {
    return this.root.isVisible();
  }

  async text(): Promise<string> {
    return ((await this.root.textContent()) ?? "").trim();
  }

  /** Attach an element screenshot to the HTML test report. */
  async screenshot(name: string): Promise<void> {
    await test.info().attach(name, {
      body: await this.root.screenshot(),
      contentType: "image/png",
    });
  }
}
