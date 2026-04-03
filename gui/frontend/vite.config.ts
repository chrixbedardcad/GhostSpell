import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";
import { copyFileSync, existsSync, readFileSync, writeFileSync, readdirSync } from "fs";

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

// Inline CSS into HTML — Wails' embedded FS doesn't reliably serve .css files.
// This makes the HTML self-contained: no external CSS request needed.
function inlineCSSPlugin() {
  return {
    name: "inline-css-into-html",
    enforce: "post" as const,
    closeBundle() {
      const dist = path.resolve(__dirname, "dist");
      const htmlPath = path.resolve(dist, "react.html");
      if (!existsSync(htmlPath)) return;

      let html = readFileSync(htmlPath, "utf-8");

      // Find all CSS files in assets/
      const assetsDir = path.resolve(dist, "assets");
      if (!existsSync(assetsDir)) return;

      const cssFiles = readdirSync(assetsDir).filter((f) => f.endsWith(".css"));
      for (const cssFile of cssFiles) {
        const cssPath = path.resolve(assetsDir, cssFile);
        const css = readFileSync(cssPath, "utf-8");

        // Remove the <link> tag referencing this CSS
        const linkPattern = new RegExp(
          `<link[^>]*href="/dist/assets/${cssFile.replace(".", "\\.")}"[^>]*>`,
          "g"
        );
        html = html.replace(linkPattern, "");

        // Inject inline <style> in <head>
        html = html.replace("</head>", `  <style>${css}</style>\n</head>`);
      }

      writeFileSync(htmlPath, html);
      console.log(`  Inlined ${cssFiles.length} CSS file(s) into react.html`);
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), copyStaticAssets(), inlineCSSPlugin()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  base: "/dist/",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    cssCodeSplit: false,
    rollupOptions: {
      input: "react.html",
    },
  },
});
