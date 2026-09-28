declare module "hoverintent" {
  type HoverIntentCallback = (event: MouseEvent) => void;

  interface HoverIntent {
    remove(): void;
  }

  function hoverIntent(
    element: HTMLElement,
    over: HoverIntentCallback,
    out: HoverIntentCallback,
  ): HoverIntent;

  export default hoverIntent;
}
