import type { Meta, StoryObj } from "@storybook/web-components-vite";
import { html } from "lit";
import { expect, waitFor } from "storybook/test";

import { lookupsSettled, spyOnFetch } from "../.storybook/fetch-spy.ts";

type ProfileArgs = {
  email?: string;
  fields?: string[];
  visible?: boolean;
  label?: string;
};

const meta = {
  title: "Web Components/omnitar-profile",
  component: "omnitar-profile",
  tags: ["autodocs"],
  argTypes: {
    email: { control: "text" },
    fields: { control: "object" },
    visible: { control: "boolean" },
    label: { control: "text", description: "Slotted fallback text" },
  },
  args: {
    visible: true,
    email: "ada@example.com",
    fields: [],
    label: "unknown@example.com",
  },
  render: (args) => html`
    <omnitar-profile
      ?visible=${args.visible}
      email=${args.email ?? ""}
      .fields=${args.fields ?? []}
      >${args.label}</omnitar-profile
    >
  `,
  beforeEach: spyOnFetch,
} satisfies Meta<ProfileArgs>;

export default meta;
type Story = StoryObj<ProfileArgs>;

const getElement = (canvasElement: HTMLElement) =>
  canvasElement.querySelector("omnitar-profile") as any;

const waitForCard = async (canvasElement: HTMLElement) => {
  const el = getElement(canvasElement);
  await waitFor(() => {
    expect(el.shadowRoot?.querySelector(".card")).toBeTruthy();
  });
  return el;
};

/** Found profile: shows the name, and the card follows the `visible` property. */
export const Card: Story = {
  play: async ({ canvasElement }) => {
    const el = await waitForCard(canvasElement);
    expect(el.shadowRoot.querySelector(".display-name")?.textContent).toContain(
      "Ada Lovelace",
    );

    const card = el.shadowRoot.querySelector(".card");
    expect(card.classList.contains("hide")).toBe(false);
    expect(card.textContent).toContain("ada@example.com");

    el.visible = false;
    await el.updateComplete;
    expect(card.classList.contains("hide")).toBe(true);
  },
};

/** Fields can be chosen by providing a csv of field names. With a fallback if name is not included */
export const SelectedFields: Story = {
  args: {
    email: "grace@example.com",
    fields: ["Title"],
    label: "Grace (fallback from slot)",
  },
  play: async ({ canvasElement }) => {
    const el = await waitForCard(canvasElement);
    expect(el.shadowRoot.querySelector(".display-name slot")).toBeTruthy();
    expect(el.shadowRoot.querySelector(".card .name")).toBeNull();
    expect(el.shadowRoot.querySelector(".card .email")).toBeNull();
  },
};

/** if a profile is not found, it shows the html provided */
export const NotFound: Story = {
  args: {
    email: "nobody@example.com",
  },
  play: async ({ canvasElement, args }) => {
    const el = getElement(canvasElement);
    await lookupsSettled();
    expect(el.shadowRoot.querySelector(".card")).toBeNull();
    expect(el.shadowRoot.querySelector("slot")).toBeTruthy();
    expect(el.textContent).toBe(args.label);
  },
};

/** Demo multiple profiles on a single page. By default two are found and third isn't */
export const MultipleIssue: StoryObj<{
  email1: string;
  email2: string;
  email3: string;
}> = {
  argTypes: {
    email1: { control: "text" },
    email2: { control: "text" },
    email3: { control: "text" },
  },
  args: {
    email1: "ada@example.com",
    email2: "grace@example.com",
    email3: "nobody@example.com",
  },
  render: (args) => html`
    email1: ${meta.render({ ...args, email: args.email1, visible: false })}<br />
    email2: ${meta.render({ ...args, email: args.email2, visible: false })}<br />
    email3: ${meta.render({ ...args, email: args.email3, visible: false })}<br />
  `,
};

/** Bug was reported where tag was embedded inside of rows with pills, and the opacity of the tag was being set to 0.5. This is a regression test to ensure that the tag is not affected by outside styles. */
export const OpacityOutsideTag: Story = {
  render: (args) => html`
    <style>
      .row {
        padding: 6px 12px;
        font: 14px sans-serif;
      }
      /* Host-page style on an ancestor of the component. */
      .muted {
        font-size: 10px;
        opacity: 0.9;
      }
    </style>

    <!-- Control: no ancestor opacity. Hover the name: the card is fully opaque. -->
    <div class="row">
      <span
        ><omnitar-profile email="${args.email}"
          >${args.email?.split("@")[0]}</omnitar-profile
        ></span
      >
    </div>

    <!-- Bug: ancestor has opacity 0.9. Hover the name: the card is ~90% opaque. -->
    <div class="row">
      <span class="muted"
        ><omnitar-profile visible email="${args.email}"
          >${args.email?.split("@")[0]}</omnitar-profile
        ></span
      >
    </div>

    <!-- Rows below the trigger, with the same name, so the bleed-through is visible behind the card. -->
    <div class="row">
      <omnitar-profile email="${args.email}"
        >${args.email?.split("@")[0]}</omnitar-profile
      >
    </div>
    <div class="row">
      <omnitar-profile email="${args.email}"
        >${args.email?.split("@")[0]}</omnitar-profile
      >
    </div>
    <div class="row">
      <omnitar-profile email="${args.email}"
        >${args.email?.split("@")[0]}</omnitar-profile
      >
    </div>
  `,
};
