import type { Meta, StoryObj } from "@storybook/web-components-vite";
import { expect, waitFor } from "storybook/test";

const revealCard = async (canvasElement: HTMLElement) => {
  const selector = "omnitar-profile";
  await waitFor(() => {
    const el = canvasElement.querySelector(selector) as any;
    expect(el?.shadowRoot?.querySelector(".card")).toBeTruthy();
  });
  const el = canvasElement.querySelector(selector) as any;
  el._show();
};

type ProfileArgs = {
  email?: string;
  source?: string;
  fields?: string;
  visible?: boolean;
};

const meta = {
  title: "Web Components/omnitar-profile",
  component: "omnitar-profile",
  tags: ["autodocs"],
  argTypes: {
    email: { control: "text" },
    source: { control: "text" },
    fields: { control: "text" },
    visible: { control: "boolean" },
  },
  args: {
    email: "ada@example.com",
    source: "slack",
    fields: "",
  },
} satisfies Meta<ProfileArgs>;

export default meta;
type Story = StoryObj<ProfileArgs>;

export const Card: Story = {
  play: async ({ canvasElement }) => {
    await revealCard(canvasElement);
  },
};

export const SelectedFields: Story = {
  args: {
    email: "grace@example.com",
    fields: "Title",
  },
  play: async ({ canvasElement }) => {
    await revealCard(canvasElement);
  },
};

export const NotFound: Story = {
  args: {
    email: "nobody@example.com",
  },
  render: (args) => {
    const el = document.createElement("omnitar-profile");
    el.setAttribute("email", String(args.email ?? ""));
    el.setAttribute("source", String(args.source ?? ""));
    if (args.visible) {
      el.setAttribute("visible", "visible");
    }
    el.textContent = "unknown@example.com";
    return el;
  },
};
