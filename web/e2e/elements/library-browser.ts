import type { Locator, Page } from "@playwright/test";
import { PageElement } from "./element";

/** Page element for src/components/LibraryBrowser.vue (toolbar + entry rows). */
export class LibraryBrowserElement extends PageElement {
  constructor(page: Page) {
    super(page.getByTestId("library-browser"));
  }

  get upButton(): Locator {
    return this.root.getByRole("button", { name: "↑ up" });
  }

  get refreshButton(): Locator {
    return this.root.getByRole("button", { name: "refresh" });
  }

  get convertFolderButton(): Locator {
    return this.root.getByRole("button", { name: "Convert this folder" });
  }

  get crumbs(): Locator {
    return this.root.locator(".crumbs");
  }

  /** Entry rows live outside the toolbar, scoped to the page like the component. */
  get rows(): Locator {
    return this.page.getByTestId("library-row");
  }

  get emptyMessage(): Locator {
    return this.page.getByText("empty");
  }

  get errorMessage(): Locator {
    return this.root.locator("+ p.error");
  }

  row(name: string): Locator {
    return this.rows.filter({ hasText: name });
  }

  async openDir(name: string): Promise<void> {
    await this.row(name).getByText(name, { exact: true }).click();
  }

  async up(): Promise<void> {
    await this.upButton.click();
  }

  async refresh(): Promise<void> {
    await this.refreshButton.click();
  }

  async convertFile(name: string): Promise<void> {
    await this.row(name).getByRole("button", { name: "Convert" }).click();
  }

  async convertThisFolder(): Promise<void> {
    await this.convertFolderButton.click();
  }
}
