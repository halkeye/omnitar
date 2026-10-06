import * as mod from "@storybook/addon-vitest/internal/coverage-reporter";

type ReporterCtor = new (opts?: unknown) => object;

const namespace = mod as unknown as {
  "module.exports"?: ReporterCtor;
  default?: ReporterCtor;
};

const ReporterClass: ReporterCtor =
  namespace["module.exports"] ?? namespace.default ?? (mod as unknown as ReporterCtor);

// The Storybook reporter expects a running Storybook test manager
// (`@storybook/addon-vitest` injects `testManager` when it launches vitest
// itself). When vitest is run from the CLI there is no manager, so the
// constructor throws; swallow that and no-op in that case.
export default class CoverageReporter {
  private inner: object | null;

  constructor(opts?: unknown) {
    try {
      this.inner = new ReporterClass(opts);
    } catch {
      this.inner = null;
    }
  }

  execute(context: unknown) {
    // With no Storybook test manager attached (plain `vitest` CLI runs)
    // the inner reporter throws when forwarding the coverage summary;
    // the report itself is still produced by the other reporters.
    try {
      return this.call("execute", context);
    } catch {
      return;
    }
  }

  onStart(root: unknown, context: unknown) {
    return this.call("onStart", root, context);
  }

  onSummary(summary: unknown) {
    return this.call("onSummary", summary);
  }

  onDetail(summary: unknown) {
    return this.call("onDetail", summary);
  }

  onEnd(root: unknown, context: unknown) {
    return this.call("onEnd", root, context);
  }

  private call(method: string, ...args: unknown[]) {
    const target = this.inner as Record<string, unknown> | null;
    const fn = target?.[method];
    if (typeof fn === "function") {
      return (fn as (...a: unknown[]) => unknown).call(target, ...args);
    }
  }
}
