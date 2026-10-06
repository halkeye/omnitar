import { describe, expect, it, vi } from "vitest";

import { installFetchMock } from "../.storybook/fetch-mock.ts";
import { csvConverter } from "./omnitar-profile.ts";
import cachedFetch from "./util-cached-fetch.ts";

const ORIGIN = "https://storybook.local";
const ACCOUNT = "STORYBOOK";

installFetchMock();

describe("csvConverter", () => {
  it("parses a comma-separated attribute into an array", () => {
    expect(csvConverter.fromAttribute("a, b ,c")).toEqual(["a", "b", "c"]);
  });

  it("returns an empty array for empty input", () => {
    expect(csvConverter.fromAttribute("")).toEqual([]);
  });

  it("serializes an array back to csv", () => {
    expect(csvConverter.toAttribute(["a", "b"])).toBe("a,b");
    expect(csvConverter.toAttribute(null)).toBe(null);
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
