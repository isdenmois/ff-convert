/// <reference types="vitest/config" />
import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: "node",
    // e2e specs (playwright) live in e2e/ and run via `bun run e2e`
    include: ["src/**/*.test.ts"],
  },
  server: {
    // E2E relies on Playwright waiting for exactly this port; fail loudly
    // instead of hopping to 5174 when it is taken.
    strictPort: true,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
