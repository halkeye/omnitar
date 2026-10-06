import { defineMain } from "@storybook/web-components-vite/node";

export default defineMain({
  framework: "@storybook/web-components-vite",
  stories: ["../webcomponent/**/*.stories.@(js|ts|mdx)"],
  addons: [
    "@storybook/addon-docs",
    "@storybook/addon-vitest",
    "@storybook/addon-a11y"
  ],
  async viteFinal(config) {
    // The project's vite.config.ts is app/webcomponent-build specific. Drop the
    // library/entry build options so Storybook can use its own iframe entry.
    if (config.build) {
      delete config.build.lib;
      delete config.build.rollupOptions;
      delete config.build.rolldownOptions;
    }
    return config;
  },
});
