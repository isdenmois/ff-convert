/// <reference types="vitest/config" />
import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

// Compiles the .vue SFCs under src/ui for component tests. The default node
// environment stays for pure-logic specs; component specs opt into happy-dom
// with a // @vitest-environment docblock.
export default defineConfig({
  plugins: [vue()],
});
