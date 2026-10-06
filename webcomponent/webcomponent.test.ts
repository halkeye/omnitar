import { afterEach, describe, expect, it, vi } from "vitest";

import { installFetchMock } from "../.storybook/fetch-mock.ts";
import registerIssue from "./omnitar-issue.ts";
import registerProfile, { csvConverter } from "./omnitar-profile.ts";
import cachedFetch from "./util-cached-fetch.ts";

const ORIGIN = "https://storybook.local";
const ACCOUNT = "STORYBOOK";

installFetchMock();
if (!customElements.get("omnitar-profile")) {
  registerProfile(ORIGIN, ACCOUNT);
}
if (!customElements.get("omnitar-issue")) {
  registerIssue(ORIGIN, ACCOUNT);
}

const mount = (html: string): HTMLElement => {
  const host = document.createElement("div");
  host.innerHTML = html;
  document.body.appendChild(host);
  return host;
};

afterEach(() => {
  document.body.innerHTML = "";
});

describe("csvConverter", () => {
  it("parses a comma-separated attribute into an array", () => {
    expect(csvConverter.fromView("a, b ,c")).toEqual(["a", "b", "c"]);
  });

  it("returns an empty array for empty input", () => {
    expect(csvConverter.fromView("")).toEqual([]);
  });

  it("serializes an array back to csv", () => {
    expect(csvConverter.toView(["a", "b"])).toBe("a,b");
    expect(csvConverter.toView(null)).toBe(null);
  });
});

describe("cachedFetch", () => {
  it("returns parsed json for ok responses and caches by url", async () => {
    const spy = vi.spyOn(globalThis, "fetch");
    const url = `${ORIGIN}/account/${ACCOUNT}/profiles/auto/ada@example.com?cachebust=1`;
    const first = await cachedFetch(url);
    const second = await cachedFetch(url);
    expect(first?.name).toBe("Ada Lovelace");
    expect(second).toBe(first);
    const calls = spy.mock.calls.filter(([input]) =>
      String(input).includes("ada@example.com"),
    );
    expect(calls).toHaveLength(1);
    spy.mockRestore();
  });
});

describe("omnitar-profile", () => {
  it("is registered as a custom element", () => {
    expect(customElements.get("omnitar-profile")).toBeDefined();
  });

  it("renders the display name and popover card once profile data loads", async () => {
    mount(
      `<omnitar-profile email="ada@example.com">fallback</omnitar-profile>`,
    );
    const el = document.querySelector("omnitar-profile") as any;

    await vi.waitFor(() => {
      expect(el.shadowRoot?.querySelector(".card")).toBeTruthy();
    });

    const name = el.shadowRoot.querySelector(".display-name");
    expect(name?.textContent).toContain("Ada Lovelace");

    el._show();
    const card = el.shadowRoot.querySelector(".card");
    expect(card.classList.contains("hide")).toBe(false);
    expect(card.textContent).toContain("ada@example.com");

    el._hide();
    expect(card.classList.contains("hide")).toBe(true);
  });

  it("falls back to slotted content when the profile is unknown", async () => {
    const host = mount(
      `<omnitar-profile email="missing@example.com">plain text</omnitar-profile>`,
    );
    const el = host.querySelector("omnitar-profile") as any;

    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(el.shadowRoot.querySelector(".card")).toBeNull();
    expect(el.shadowRoot.querySelector("slot")).toBeTruthy();
    expect(el.textContent).toBe("plain text");
  });
});

describe("omnitar-issue", () => {
  it("is registered as a custom element", () => {
    expect(customElements.get("omnitar-issue")).toBeDefined();
  });

  it("renders issue key, title and status once data loads", async () => {
    mount(`<omnitar-issue issue="HR-1">HR-1</omnitar-issue>`);
    const el = document.querySelector("omnitar-issue") as any;

    await vi.waitFor(() => {
      expect(el.shadowRoot?.querySelector(".card")).toBeTruthy();
    });

    const card = el.shadowRoot.querySelector(".card");
    expect(card.textContent).toContain("Add hover cards to the profile page");

    const status = el.shadowRoot.querySelector(".status-pill");
    expect(status?.textContent).toContain("In Progress");
  });

  it("falls back to slotted content for unknown issues", async () => {
    mount(`<omnitar-issue issue="HR-999">HR-999</omnitar-issue>`);
    const el = document.querySelector("omnitar-issue") as any;

    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(el.shadowRoot.querySelector(".card")).toBeNull();
    expect(el.shadowRoot.querySelector("slot")).toBeTruthy();
  });
});
