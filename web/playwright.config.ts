import { defineConfig, devices } from "@playwright/test";

/**
 * E2E suite for the SPA against the mock API (e2e/mock/server.ts): the mock
 * owns port 8080, Vite dev serves the SPA on 5173 and proxies /api (incl. the
 * WS snapshot feed) to the mock. Serial, single worker: one shared mock.
 */
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  reporter: [["html", { open: "never" }]],
  use: {
    baseURL: "http://localhost:5173",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      command: "bun e2e/mock/server.ts",
      url: "http://localhost:8080/api/jobs",
      reuseExistingServer: false,
      stdout: "ignore",
    },
    {
      command: "bun run dev",
      url: "http://localhost:5173",
      reuseExistingServer: true,
      stdout: "ignore",
    },
  ],
});
