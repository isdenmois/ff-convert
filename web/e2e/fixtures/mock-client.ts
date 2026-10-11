import { test as base } from "@playwright/test";
import type { JobTemplate, LibraryTree, RecordedRequest } from "../mock/state";
import { MainPage } from "../pages/main-page";

const MOCK_URL = process.env.FFCONVERT_E2E_MOCK ?? "http://localhost:8080";

/** Typed client for the mock's test-only control plane (/api/__test/*). */
export class MockClient {
  async reset(): Promise<void> {
    await this.post("/api/__test/reset");
  }

  async seedLibrary(tree: LibraryTree): Promise<void> {
    await this.post("/api/__test/seed-library", { tree });
  }

  async seedJob(template: JobTemplate, state?: "questions"): Promise<void> {
    await this.post("/api/__test/seed-job", { template, state });
  }

  /** REST calls the SPA made against the mock (for payload assertions). */
  async requests(): Promise<RecordedRequest[]> {
    const res = await fetch(`${MOCK_URL}/api/__test/requests`);
    if (!res.ok) throw new Error(`mock requests failed: ${res.status}`);
    return res.json();
  }

  private async post(path: string, body?: unknown): Promise<unknown> {
    const res = await fetch(`${MOCK_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body ?? {}),
    });
    if (!res.ok) throw new Error(`mock ${path} failed: ${res.status}`);
    return res.json();
  }
}

/**
 * Playwright `test` extended with the mock control client (state is reset
 * around every test) and a ready-made MainPage object.
 */
export const test = base.extend<{ mock: MockClient; mainPage: MainPage }>({
  // biome-ignore lint/correctness/noEmptyPattern: playwright requires the object destructuring pattern
  mock: async ({}, use) => {
    const client = new MockClient();
    await client.reset();
    await use(client);
    await client.reset();
  },
  mainPage: async ({ page }, use) => {
    await use(new MainPage(page));
  },
});

export { expect } from "@playwright/test";
