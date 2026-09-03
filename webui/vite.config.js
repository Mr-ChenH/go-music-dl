import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: resolve(__dirname, "../internal/web/templates/static/react"),
    emptyOutDir: true,
    cssCodeSplit: false,
    rollupOptions: {
      input: resolve(__dirname, "src/main.jsx"),
      output: {
        entryFileNames: "react-app.js",
        assetFileNames: (assetInfo) =>
          assetInfo.name?.endsWith(".css") ? "react-app.css" : "[name][extname]",
      },
    },
  },
});
