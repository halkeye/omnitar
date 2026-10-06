import type { Preview } from "@storybook/web-components-vite";
import { INITIAL_VIEWPORTS } from "storybook/viewport";

import registerIssue from "../webcomponent/omnitar-issue.ts";
import registerProfile from "../webcomponent/omnitar-profile.ts";
import { installFetchMock } from "./fetch-mock.ts";

installFetchMock();

const STORYBOOK_ORIGIN = "https://storybook.local";
const STORYBOOK_ACCOUNT = "STORYBOOK";

if (!customElements.get("omnitar-profile")) {
  registerProfile(STORYBOOK_ORIGIN, STORYBOOK_ACCOUNT);
}
if (!customElements.get("omnitar-issue")) {
  registerIssue(STORYBOOK_ORIGIN, STORYBOOK_ACCOUNT);
}

const preview: Preview = {
  parameters: {
    viewport: {
      options: INITIAL_VIEWPORTS,
    },
    controls: {
      matchers: { color: /(background|color)$/i, date: /Date$/i },
    },
  },
  initialGlobals: {
    viewport: { value: "ipad", isRotated: false },
  },
};

export default preview;
