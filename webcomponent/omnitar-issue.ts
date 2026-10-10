// <omnitar-issue> web component
//
// Usage:
//   <omnitar-issue issue="HR-1">HR-1</omnitar-issue>
//

import { css, html, LitElement, type PropertyValues } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { map } from "lit/directives/map.js";
import { live } from "lit/directives/live.js";
import cachedFetch from "./util-cached-fetch.ts";
import hoverintent from "hoverintent";
import svgJira from "./assets/jira.svg";

let scriptOrigin = "";
let accountUUID = "";

function channelLuminance(value: number) {
  const c = value / 255;
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

function relativeLuminance(color: string): number | null {
  const hex = color.trim().replace(/^#/, "");
  const expanded =
    hex.length === 3
      ? hex
          .split("")
          .map((c) => c + c)
          .join("")
      : hex;
  if (!/^[0-9a-f]{6}$/i.test(expanded)) {
    return null;
  }
  const [r, g, b] = [0, 2, 4].map((i) =>
    parseInt(expanded.slice(i, i + 2), 16),
  );
  return (
    0.2126 * channelLuminance(r) +
    0.7152 * channelLuminance(g) +
    0.0722 * channelLuminance(b)
  );
}

export function statusTextColor(backgroundColor: string): string {
  const luminance = relativeLuminance(backgroundColor);
  if (luminance === null) {
    return "#1f2937";
  }
  const contrastBlack = (luminance + 0.05) / 0.05;
  const contrastWhite = 1.05 / (luminance + 0.05);
  return contrastWhite >= contrastBlack ? "#ffffff" : "#111111";
}

class Issue {
  url: string = "";
  title: string = "";
  key: string = "";
  issue_type: string = "";
  issue_type_icon: string = "";
  reporter: string = "";
  reporter_icon: string = "";
  assignee: string = "";
  assignee_icon: string = "";
  status: string = "";
  status_color: string = "";
  priority: string = "";
  priority_icon: string = "";
  fields: {
    [key: string]: string;
  } = {};
}

const styles = css`
  :host {
    position: relative;
    display: inline-block;
  }

  :host([has-data]) {
    text-decoration-line: underline;
    text-decoration-style: dashed;
    text-decoration-color: #9ca3af;
    text-underline-offset: 2px;
  }

  .display-name {
    anchor-name: --omnitar-issue-trigger;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    text-decoration-line: underline;
    text-decoration-style: dashed;
    text-decoration-color: #9ca3af;
    text-underline-offset: 2px;
  }

  .display-name img {
    width: 1em;
    height: 1em;
  }

  .card {
    /* Reset UA [popover] { position: fixed; inset: 0; margin: auto } */
    position: fixed;
    inset: unset;
    margin: 0;
    position-anchor: --omnitar-issue-trigger;
    top: calc(anchor(bottom) + 8px);
    left: anchor(left);
    display: block;
    box-sizing: border-box;
    padding: 14px;
    width: min(360px, calc(100vw - 24px));
    min-width: 240px;
    background: #fff;
    color: #1b1b1b;
    border: 1px solid #e4e4e7;
    border-radius: 10px;
    box-shadow:
      0 8px 24px rgba(0, 0, 0, 0.12),
      0 2px 6px rgba(0, 0, 0, 0.08);
    font:
      13px/1.4 -apple-system,
      BlinkMacSystemFont,
      "Segoe UI",
      Roboto,
      sans-serif;
    white-space: normal;
    z-index: 2147483647;
    opacity: 1;
    transform: translateY(0);
    transition:
      opacity 120ms ease,
      transform 120ms ease;
    pointer-events: auto;
  }

  .card.hide {
    opacity: 0;
    transform: translateY(-4px);
    pointer-events: none;
  }

  :host([visible]) .card {
    transition: none;
  }

  /* little pointer/caret at the top of the card */
  .card::before {
    content: "";
    position: absolute;
    top: -6px;
    left: 16px;
    width: 10px;
    height: 10px;
    background: #fff;
    border-left: 1px solid #e4e4e7;
    border-top: 1px solid #e4e4e7;
    transform: rotate(45deg);
  }

  .issue-heading {
    display: flex;
    align-items: flex-start;
    gap: 8px;
  }

  .issue-type-icon {
    flex: none;
    width: 20px;
    height: 20px;
    margin-top: 1px;
    object-fit: contain;
  }

  .issue-summary {
    min-width: 0;
  }

  .issue-key {
    margin-right: 6px;
    color: #6b7280;
    font-size: 12px;
    font-weight: 600;
    line-height: 1.3;
    white-space: nowrap;
  }

  .issue-title {
    font-size: 14px;
    font-weight: 600;
    line-height: 1.35;
  }

  .issue-meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
    margin-top: 12px;
    color: #4b5563;
    font-size: 12px;
  }

  .meta-item {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
  }

  .meta-icon {
    flex: none;
    width: 16px;
    height: 16px;
    object-fit: contain;
  }

  .status-pill {
    display: inline-flex;
    align-items: center;
    min-height: 20px;
    box-sizing: border-box;
    padding: 2px 8px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 600;
    line-height: 1.2;
    white-space: nowrap;
  }

  .card dl.fields {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 4px 8px;
    margin: 12px 0;
    font-size: 12px;
    color: #444;
  }

  .card dl.fields dt {
    font-weight: 500;
    color: #888;
  }

  .card dl.fields dd {
    margin: 0;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .button-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 30px;
    box-sizing: border-box;
    padding: 5px 12px;
    font-size: 12px;
    font-weight: 600;
    background-color: #4a154b; /* Slack purple */
    color: white; /* Text color */
    text-decoration: none; /* Remove underline */
    border-radius: 6px; /* Rounded corners */
    text-align: center; /* Center text */
    transition: background-color 120ms ease;
  }

  .button-link:hover {
    background-color: #611f69; /* Darker shade on hover */
  }

  .button-link:focus-visible {
    outline: 2px solid #4a154b;
    outline-offset: 2px;
  }
`;

/**
 * Hover card for an issue, looked up by key.
 */
@customElement("omnitar-issue")
export class OmnitarIssueElement extends LitElement {
  static styles = styles;

  /** Not yet working, but will be used to support multiple issue sources in the future. */
  @property({ type: String })
  source: string = "auto";

  /** Issue key to display in the card. If not provided, the card will not be displayed. */
  @property({ type: String })
  issue?: string;

  /** When set, keeps the card open. When unset, the card opens on hover only. */
  @property({ type: Boolean, reflect: true })
  visible = false;

  /** @internal */
  @state()
  issueData: Issue | null = null;

  /** @internal */
  @state()
  private _hoverOpen = false;

  /** @internal */
  private _hoverIntent: ReturnType<typeof hoverintent> | null = null;

  /** @internal */
  @query(".display-name")
  private trigger?: HTMLElement;

  /** @internal */
  @query(".card")
  private card?: HTMLElement;

  /** @internal */
  private get _cardOpen(): boolean {
    return this.visible || this._hoverOpen;
  }

  willUpdate(changed: PropertyValues<this>) {
    if (changed.has("issue")) {
      const issue = this.issue;
      if (!issue) {
        this.issueData = null;
        return;
      }
      cachedFetch(
        `${scriptOrigin}/account/${encodeURI(accountUUID)}/issues/${encodeURI(this.source)}/${encodeURI(issue)}`,
      ).then((issueData) => {
        this.issueData = issueData;
      });
    }
  }

  updated(changed: PropertyValues<this>) {
    if (changed.has("issueData")) {
      this.toggleAttribute("has-data", !!this.issueData);
    }
    if (
      this.issueData &&
      (changed.has("issueData") ||
        changed.has("visible") ||
        changed.has("_hoverOpen"))
    ) {
      this._syncCardOpen();
    }
  }

  connectedCallback() {
    super.connectedCallback();
    this._hoverIntent = hoverintent(this, this._show, this._hide);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._hoverOpen = false;
    this._closeCard();
    this._hoverIntent?.remove();
  }

  /** @internal */
  private static _anchorPositioningSupported =
    typeof CSS !== "undefined" && CSS.supports("top", "anchor(bottom)");

  /** @internal */
  private _positionCard = () => {
    if (OmnitarIssueElement._anchorPositioningSupported) {
      return;
    }
    if (!this.card || !this.trigger) {
      return;
    }
    const { bottom, left } = this.trigger.getBoundingClientRect();
    this.card.style.top = `${bottom + 8}px`;
    this.card.style.left = `${left}px`;
  };

  /** @internal */
  private _syncCardOpen = () => {
    if (!this.card || !this.issueData) {
      return;
    }
    if (this._cardOpen) {
      this._positionCard();
      if (!this.card.matches(":popover-open")) {
        this.card.showPopover();
      }
    } else {
      this._closeCard();
    }
  };

  /** @internal */
  private _closeCard = () => {
    if (!this.card?.matches(":popover-open")) {
      return;
    }
    this.card.hidePopover();
  };

  /** @internal */
  private _show = () => {
    if (this.visible) {
      return;
    }
    this._hoverOpen = true;
  };

  /** @internal */
  private _hide = () => {
    if (this.visible) {
      return;
    }
    this._hoverOpen = false;
  };

  render() {
    if (!this.issueData) {
      return html`<slot></slot>`;
    }
    const issue = this.issueData;
    return html`
      <span class="display-name">
        <img class="icon" src=${live(svgJira)} alt="Jira" />
        ${issue.key}
      </span>
      <div class="card ${this._cardOpen ? "" : "hide"}" popover="manual">
        <div class="issue-heading">
          <img
            class="issue-type-icon"
            @error=${() => {
              this.issueData = { ...this.issueData!, issue_type_icon: svgJira };
            }}
            src="${issue.issue_type_icon}"
            alt="${issue.issue_type}"
          />
          <div class="issue-summary">
            <span class="issue-key">${issue.key}</span>
            <span class="issue-title">${issue.title}</span>
          </div>
        </div>
        <div class="issue-meta">
          <span class="meta-item">
            <img
              class="meta-icon"
              src="${issue.reporter_icon}"
              alt="Reporter: ${issue.reporter}"
            />
            ${issue.reporter}
          </span>
          <span
            class="status-pill"
            style="background-color: ${issue.status_color}; color: ${statusTextColor(
              issue.status_color,
            )}"
          >
            ${issue.status}
          </span>
          <span class="meta-item">
            <img
              class="meta-icon"
              @error=${() => {
                this.issueData = { ...this.issueData!, priority_icon: svgJira };
              }}
              src="${issue.priority_icon}"
              alt="Priority: ${issue.priority}"
            />
            ${issue.priority}
          </span>
        </div>
        <dl class="fields">
          ${map(
            Object.entries(issue.fields),
            ([key, value]) => html`
              <dt>${key}</dt>
              <dd>${value}</dd>
            `,
          )}
        </dl>
        <a
          class="message-link button-link"
          href="${issue.url}"
          target="_blank"
          rel="noopener noreferrer"
          >View issue</a
        >
      </div>
    `;
  }
}

export function register(o: string, a: string) {
  scriptOrigin = o;
  accountUUID = a;
}

export default register;
