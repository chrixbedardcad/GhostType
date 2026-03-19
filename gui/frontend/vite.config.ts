import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";
import { copyFileSync, mkdirSync, existsSync, readdirSync } from "fs";

// Copy static assets (images) into dist after build
function copyStaticAssets() {
  return {
    name: "copy-static-assets",
    closeBundle() {
      const dist = path.resolve(__dirname, "dist");
      const assets = ["ghost-icon.png", "ghostspell-ghost.png", "ghostAI.png",
                       "ghostspell-cloud.svg", "ghostspell-local.svg"];
      for (const file of assets) {
        const src = path.resolve(__dirname, file);
        if (existsSync(src)) {
          copyFileSync(src, path.resolve(dist, file));
        }
      }
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), copyStaticAssets()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rollupOptions: {
      input: "react.html",
    },
  },
});
