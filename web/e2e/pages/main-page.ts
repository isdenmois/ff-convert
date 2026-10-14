import { expect, type Locator } from "@playwright/test";
import { AnswersModalElement } from "../elements/answers-modal";
import { JobQueueElement } from "../elements/job-card";
import { LibraryBrowserElement } from "../elements/library-browser";
import { AppPage } from "../elements/page";

/**
 * The app's single screen: header, library browser, job queue and (conditionally)
 * the answers modal. Composed from page elements — specs never touch raw locators.
 */
export class MainPage extends AppPage {
  get library(): LibraryBrowserElement {
    return new LibraryBrowserElement(this.page);
  }

  get jobs(): JobQueueElement {
    return new JobQueueElement(this.page);
  }

  get modal(): AnswersModalElement {
    return new AnswersModalElement(this.page);
  }

  get connection(): Locator {
    return this.page.getByTestId("conn-status");
  }

  get errorBanner(): Locator {
    return this.page.locator("header ~ p.error");
  }

  /** Opens the app and waits for the WS snapshot feed to come alive. */
  async open(): Promise<void> {
    await super.open("/");
    await expect(this.page.getByRole("heading", { name: "Queue" })).toBeVisible();
  }
}
