import { resolve } from "node:path";
import { defineConfig } from "vite";

/** @type {import('vite').UserConfig} */
export default defineConfig(({ command, mode }) => {
  const buildPage = mode === "page";

  return {
    root: "webcomponent",
    plugins: [
      {
        name: "html-inject-nonce-into-script-tag",
        enforce: "post",
        transformIndexHtml(html: string) {
          if (command === "build") {
            html = html.replace(
              "</body>",
              `<script type="text/javascript">
              const scriptTag = document.createElement('script');
              scriptTag.setAttribute('src','/webcomponent.js?orgId=E0AFSMB25HU');
              scriptTag.setAttribute('type','module');
              document.head.appendChild(scriptTag);
            </script>`,
            );
          }
          return html;
        },
      },
    ],
    build: {
      sourcemap: true,
      outDir: "../static",
      target: "es2020",
      // static is outside Vite's root and includes this repository's .gitkeep.
      emptyOutDir: false,
      ...(buildPage
        ? {
            rollupOptions: {
              input: resolve(import.meta.dirname, "webcomponent/index.html"),
            },
          }
        : {
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
          }),
    },
  };
});
