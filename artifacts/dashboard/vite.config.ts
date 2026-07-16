import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

// PORT and BASE_PATH are dev/preview-server concerns and are irrelevant to a
// static production build (Vercel). Default them instead of throwing, so
// `vite build` succeeds in CI/Vercel where they aren't set.
const port = Number(process.env.PORT) || 5173;
const basePath = process.env.BASE_PATH || "/";

const isDev = process.env.NODE_ENV !== "production";
const onReplit = process.env.REPL_ID !== undefined;

export default defineConfig(async () => {
  const plugins = [react(), tailwindcss()];

  // Replit-only dev tooling — never ship these to end users.
  if (isDev && onReplit) {
    const [{ default: runtimeErrorOverlay }, cartographer, devBanner] =
      await Promise.all([
        import("@replit/vite-plugin-runtime-error-modal"),
        import("@replit/vite-plugin-cartographer"),
        import("@replit/vite-plugin-dev-banner"),
      ]);
    plugins.push(
      runtimeErrorOverlay(),
      cartographer.cartographer({ root: path.resolve(import.meta.dirname, "..") }),
      devBanner.devBanner(),
    );
  }

  return {
    base: basePath,
    plugins,
    resolve: {
      alias: {
        "@": path.resolve(import.meta.dirname, "src"),
        "@assets": path.resolve(import.meta.dirname, "..", "..", "attached_assets"),
      },
      dedupe: ["react", "react-dom"],
    },
    root: path.resolve(import.meta.dirname),
    build: {
      outDir: path.resolve(import.meta.dirname, "dist/public"),
      emptyOutDir: true,
      // Split heavy libraries into their own chunks so the initial bundle
      // stays small and vendor code caches independently of app code.
      rollupOptions: {
        output: {
          manualChunks: {
            "react-vendor": ["react", "react-dom", "wouter"],
            charts: ["recharts"],
            motion: ["framer-motion"],
          },
        },
      },
    },
    server: {
      port,
      strictPort: true,
      host: "0.0.0.0",
      allowedHosts: true,
      fs: { strict: true },
      proxy: {
        "/api": {
          target: "http://localhost:8080",
          changeOrigin: true,
          ws: true,
        },
      },
    },
    preview: {
      port,
      host: "0.0.0.0",
      allowedHosts: true,
    },
  };
});
