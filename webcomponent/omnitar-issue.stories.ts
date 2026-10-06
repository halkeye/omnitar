import type { Meta, StoryObj } from "@storybook/web-components-vite";
import { html } from "lit";
import { spyOn } from "storybook/test";

type IssueArgs = {
  issue?: string;
  source?: string;
};

const meta = {
  title: "Web Components/omnitar-issue",
  component: "omnitar-issue",
  tags: ["autodocs"],
  argTypes: {
    issue: { control: "text" },
    source: { control: "text" },
  },
  args: {
    issue: "HR-1",
    source: "jira",
  },
  render: (args) => html`
    <omnitar-issue visible issue=${args.issue ?? ""} source=${args.source ?? ""}
      >${args.issue ?? "HR-999"}</omnitar-issue
    >
  `,
  async beforeEach() {
    spyOn(console, "log").mockName("console.log");
  },
} satisfies Meta<IssueArgs>;

export default meta;
type Story = StoryObj<IssueArgs>;

export const Card: Story = {};

export const SecondIssue: Story = {
  args: {
    issue: "HR-2",
  },
};

export const NotFound: Story = {
  args: {
    issue: "HR-999",
  },
};
