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

// Inline CSS into HTML and unwrap @layer blocks for WebView2 compatibility.
// Tailwind v4 generates CSS cascade layers (@layer) which some WebView2 versions
// don't fully support. Unwrapping extracts the rules into plain CSS.
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
        let css = readFileSync(cssPath, "utf-8");

        // Unwrap @layer blocks — extract content, discard the @layer wrapper.
        // This fixes WebView2 compatibility where @layer rules are ignored.
        css = unwrapLayers(css);

        // Remove the <link> tag referencing this CSS
        const linkPattern = new RegExp(
          `<link[^>]*href="/dist/assets/${cssFile.replace(/\./g, "\\.")}"[^>]*>`,
          "g"
        );
        html = html.replace(linkPattern, "");

        // Inject inline <style> in <head>
        html = html.replace("</head>", `  <style>${css}</style>\n</head>`);
      }

      writeFileSync(htmlPath, html);
      console.log(`  Inlined ${cssFiles.length} CSS file(s) into react.html (layers unwrapped)`);
    },
  };
}

// Unwrap @layer declarations — replace "@layer name { ... }" with just "..."
// Handles nested braces correctly.
function unwrapLayers(css: string): string {
  let result = "";
  let i = 0;
  while (i < css.length) {
    // Check for @layer
    if (css.startsWith("@layer", i)) {
      // Find the opening brace
      const braceStart = css.indexOf("{", i);
      if (braceStart === -1) { result += css.slice(i); break; }
      // Extract content between balanced braces
      let depth = 1;
      let j = braceStart + 1;
      while (j < css.length && depth > 0) {
        if (css[j] === "{") depth++;
        else if (css[j] === "}") depth--;
        j++;
      }
      // Inner content (without the outer braces)
      result += css.slice(braceStart + 1, j - 1);
      i = j;
    } else {
      result += css[i];
      i++;
    }
  }
  return result;
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
