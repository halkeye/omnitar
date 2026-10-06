import { spyOn, type MockInstance } from "storybook/test";

import { clearCache } from "../webcomponent/util-cached-fetch.ts";

let spy: MockInstance<typeof fetch> | undefined;

/**
 * Call from a story's `beforeEach` (it runs before render). Spies on the mocked
 * `fetch` and clears the lookup cache so a replayed story fetches again.
 */
export function spyOnFetch() {
  clearCache();
  spy = spyOn(globalThis, "fetch");
}

/** Resolves once every lookup started so far has finished and the element had a chance to react. */
export async function lookupsSettled() {
  await Promise.all(spy?.mock.results.map((r) => r.value) ?? []);
  // cachedFetch/willUpdate continue in a few microtasks after fetch resolves.
  await new Promise((resolve) => setTimeout(resolve, 0));
}
