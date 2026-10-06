import type { Meta, StoryObj } from "@storybook/web-components-vite";
import { html } from "lit";
import { expect, spyOn, waitFor } from "storybook/test";

import { lookupsSettled, spyOnFetch } from "../.storybook/fetch-spy.ts";

type IssueArgs = {
  issue?: string;
  visible?: boolean;
};

const meta = {
  title: "Web Components/omnitar-issue",
  component: "omnitar-issue",
  tags: ["autodocs"],
  argTypes: {
    issue: { control: "text" },
    visible: { control: "boolean" },
  },
  args: {
    visible: true,
    issue: "HR-1",
  },
  render: (args) => html`
    <omnitar-issue ?visible=${args.visible} issue=${args.issue ?? ""}
      >${args.issue ?? "HR-999"}</omnitar-issue
    >
  `,
  async beforeEach() {
    spyOn(console, "log").mockName("console.log");
    spyOnFetch();
  },
} satisfies Meta<IssueArgs>;

export default meta;
type Story = StoryObj<IssueArgs>;

/** Demo of fully populated issue card. By default HR-1 is found and displayed */
export const Card: Story = {
  play: async ({ canvasElement }) => {
    const el = canvasElement.querySelector("omnitar-issue") as any;
    await waitFor(() => {
      expect(el.shadowRoot?.querySelector(".card")).toBeTruthy();
    });
    expect(el.shadowRoot.querySelector(".card").textContent).toContain(
      "Add hover cards to the profile page",
    );
    expect(el.shadowRoot.querySelector(".status-pill")?.textContent).toContain(
      "In Progress",
    );
  },
};

/** Demo multiple issues on a single page. By default two are found and third isn't */
export const MultipleIssue: StoryObj<{
  issue1: string;
  issue2: string;
  issue3: string;
}> = {
  argTypes: {
    issue1: { control: "text" },
    issue2: { control: "text" },
    issue3: { control: "text" },
  },
  args: {
    issue1: "HR-1",
    issue2: "HR-2",
    issue3: "HR-3",
  },
  render: (args) => html`
    issue1: ${meta.render({ ...args, issue: args.issue1, visible: false })}<br />
    issue2: ${meta.render({ ...args, issue: args.issue2, visible: false })}<br />
    issue3: ${meta.render({ ...args, issue: args.issue3, visible: false })}<br />
  `,
};

/** When a provided issue is not found, just show the provided html inside the element */
export const NotFound: Story = {
  args: {
    issue: "HR-999",
  },
  play: async ({ canvasElement }) => {
    const el = canvasElement.querySelector("omnitar-issue") as any;
    await lookupsSettled();
    expect(el.shadowRoot.querySelector(".card")).toBeNull();
    expect(el.shadowRoot.querySelector("slot")).toBeTruthy();
  },
};
