const cache = new Map<string, Promise<any | null>>();

/** Drops memoised lookups. Used by Storybook so replayed stories re-fetch. */
export const clearCache = () => cache.clear();

export const cachedFetch = async (url: string) => {
  if (!cache.has(url)) {
    cache.set(
      url,
      (async (url) => {
        const res = await fetch(url);
        if (res.ok) {
          return await res.json();
        }
        return null;
      })(url),
    );
  }

  const getter = cache.get(url);
  if (!getter) {
    return null;
  }
  return await getter;
};

export default cachedFetch;
