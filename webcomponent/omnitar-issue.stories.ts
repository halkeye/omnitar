import type { Meta, StoryObj } from "@storybook/web-components-vite";
import { expect, waitFor } from "storybook/test";

const revealCard = async (canvasElement: HTMLElement) => {
  const selector = "omnitar-issue";
  await waitFor(() => {
    const el = canvasElement.querySelector(selector) as any;
    expect(el?.shadowRoot?.querySelector(".card")).toBeTruthy();
  });
  const el = canvasElement.querySelector(selector) as any;
  el._show();
};

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
} satisfies Meta<IssueArgs>;

export default meta;
type Story = StoryObj<IssueArgs>;

export const Card: Story = {
  play: async ({ canvasElement }) => {
    await revealCard(canvasElement);
  },
};

export const SecondIssue: Story = {
  args: {
    issue: "HR-2",
  },
  play: async ({ canvasElement }) => {
    await revealCard(canvasElement);
  },
};

export const NotFound: Story = {
  args: {
    issue: "HR-999",
  },
  render: (args) => {
    const el = document.createElement("omnitar-issue");
    el.setAttribute("issue", String(args.issue ?? ""));
    el.setAttribute("source", String(args.source ?? ""));
    el.textContent = "HR-999";
    return el;
  },
};
