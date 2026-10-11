import { fileURLToPath, URL } from "node:url";
import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  optimizeDeps: {
    // Workspace-linked source package (ships .vue SFCs); never pre-bundle it.
    exclude: ["@ffconvert/shared"],
  },
  server: {
    fs: {
      // Serve the shared workspace package living outside frontend/.
      allow: [fileURLToPath(new URL("../../..", import.meta.url))],
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
