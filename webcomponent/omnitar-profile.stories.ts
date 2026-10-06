import type { Meta, StoryObj } from "@storybook/web-components-vite";
import { html } from "lit";

type ProfileArgs = {
  email?: string;
  source?: string;
  fields?: string[];
  visible?: boolean;
};

const meta = {
  title: "Web Components/omnitar-profile",
  component: "omnitar-profile",
  tags: ["autodocs"],
  argTypes: {
    email: { control: "text" },
    source: { control: "text" },
    fields: { control: "object" },
    visible: { control: "boolean" },
  },
  args: {
    visible: true,
    email: "ada@example.com",
    source: "slack",
    fields: [],
  },
  render: (args) => html`
    <omnitar-profile
      ?visible=${args.visible}
      email=${args.email ?? ""}
      source=${args.source ?? ""}
      .fields=${args.fields ?? []}
      >unknown@example.com</omnitar-profile
    >
  `,
} satisfies Meta<ProfileArgs>;

export default meta;
type Story = StoryObj<ProfileArgs>;

export const Card: Story = {};

export const SelectedFields: Story = {
  args: {
    email: "grace@example.com",
    fields: ["Title"],
  },
};

export const NotFound: Story = {
  args: {
    email: "nobody@example.com",
  },
};
