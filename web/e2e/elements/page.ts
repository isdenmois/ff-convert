import { type Page, test } from "@playwright/test";

/** Base class for Page Objects: one per distinct screen/route of the app. */
export abstract class AppPage {
  constructor(readonly page: Page) {}

  async open(path = "/"): Promise<void> {
    await this.page.goto(path);
  }

  /** Attach a full-page screenshot to the HTML test report. */
  async screenshot(name: string): Promise<void> {
    await test.info().attach(name, {
      body: await this.page.screenshot({ fullPage: true }),
      contentType: "image/png",
    });
  }
}
