import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import { playwright } from "@vitest/browser-playwright";
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    projects: [
      {
        plugins: [storybookTest({ configDir: ".storybook" })],
        test: {
          name: "storybook",
          setupFiles: [".storybook/vitest.setup.ts"],
          browser: {
            enabled: true,
            headless: true,
            provider: playwright(),
            instances: [{ browser: "chromium", name: "storybook-chromium" }],
          },
        },
      },
      {
        test: {
          name: "unit",
          include: ["webcomponent/**/*.test.ts"],
          browser: {
            enabled: true,
            headless: true,
            provider: playwright(),
            instances: [{ browser: "chromium", name: "unit-chromium" }],
          },
        },
      },
    ],
  },
});
