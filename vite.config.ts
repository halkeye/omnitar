import { resolve } from "node:path";
import { defineConfig } from "vite";

export default defineConfig(({ mode }) => {
  const isWebComponent = mode === "webcomponent";

  return {
    build: {
      outDir: "static",
      emptyOutDir: false,
      target: "es2020",
      ...(isWebComponent
        ? {
            lib: {
              entry: resolve(
                import.meta.dirname,
                "webcomponent/webcomponent.ts",
              ),
              formats: ["es"],
              fileName: "webcomponent",
            },
            rollupOptions: {
              output: {
                codeSplitting: false,
              },
            },
          }
        : {
            cssCodeSplit: false,
            rollupOptions: {
              input: resolve(import.meta.dirname, "app/app.js"),
              output: {
                entryFileNames: "js/app.js",
                assetFileNames: (asset) =>
                  asset.name?.endsWith(".css")
                    ? "css/index.css"
                    : "assets/[name]-[hash][extname]",
              },
            },
          }),
    },
  };
});
