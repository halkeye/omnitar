// <omnitar-issue> web component
//
// Usage:
//   <omnitar-issue issue="HR-1">HR-1</omnitar-issue>
//

import {
  attr,
  css,
  FASTElement,
  html,
  observable,
  repeat,
  when,
} from "@microsoft/fast-element";
import cachedFetch from "./util-cached-fetch.ts";
import hoverIntent from "hoverintent";

import { enableDebug } from "@microsoft/fast-element/debug.js";
import svgJira from "./assets/jira.svg";

enableDebug();

let scriptOrigin = "";
let accountUUID = "";

function channelLuminance(value: number) {
  const c = value / 255;
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

function relativeLuminance(color: string): number | null {
  const hex = color.trim().replace(/^#/, "");
  const expanded =
    hex.length === 3 ? hex.split("").map((c) => c + c).join("") : hex;
  if (!/^[0-9a-f]{6}$/i.test(expanded)) {
    return null;
  }
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(expanded.slice(i, i + 2), 16));
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

class OmnitarIssueElement extends FASTElement {
  @attr
  source: string = "auto";

  @attr
  issue?: string;

  @attr
  visible?: Boolean | undefined;

  @observable
  issueData: Issue | null = null;

  private _hoverIntent: ReturnType<typeof hoverIntent> | null = null;

  issueDataChanged(_: Issue, newValue?: Issue) {
    if (newValue) {
      this.$fastController.addStyles(cssHasProfile);
    } else {
      this.$fastController.addStyles(cssNoProfile);
    }
  }

  issueChanged(_: string, newValue?: string) {
    if (!newValue) {
      this.issueData = null;
      return;
    }

    cachedFetch(
      `${scriptOrigin}/account/${encodeURI(accountUUID)}/issues/${encodeURI(this.source)}/${encodeURI(newValue)}`,
    ).then((issueData) => {
      this.issueData = issueData;
    });
  }

  visibleChanged(oldValue: Boolean, newValue?: Boolean) {
    if (newValue) {
      this._show();
    } else if (oldValue !== newValue) {
      this._hide();
    }
  }

  connectedCallback() {
    super.connectedCallback();
    if (!this) {
      return;
    }
    this._hoverIntent = hoverIntent(this, this._show, this._hide);
  }

  disconnectedCallback() {
    this._hide();
    this._hoverIntent?.remove();
  }

  _show = async () => {
    if (!this.issueData) {
      return;
    }
    if (this.shadowRoot) {
      this.shadowRoot.querySelector(".card")?.classList.remove("hide");
    }
  };

  _hide = () => {
    if (!this.issueData) {
      return;
    }
    if (this.shadowRoot) {
      this.shadowRoot.querySelector(".card")?.classList.add("hide");
    }
  };
}

const cssGlobal = css`
  .hide {
    display: none !important;
  }
`;
const cssNoProfile = css``;
const cssHasProfile = css`
  :host {
    position: relative;
    display: inline-block;
    text-decoration-line: underline;
    text-decoration-style: dashed;
    text-decoration-color: #9ca3af;
    text-underline-offset: 2px;
  }

  .display-name {
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
    position: absolute;
    top: 75%;
    left: 0;
    margin-top: 8px;
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

  /* override the generic .hide (display:none) so we can animate instead */
  .card.hide {
    display: block !important;
    opacity: 0;
    transform: translateY(-4px);
    pointer-events: none;
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

const issueTemplateHTML = html<OmnitarIssueElement>`
  <template>
    ${when(
      (x) => !x.issueData,
      html<OmnitarIssueElement>`<slot></slot>`,
      html<OmnitarIssueElement>`
        <span class="display-name">
          <img class="icon" src=${svgJira} alt="Jira" />
          ${(x) => x.issueData!.key}
        </span>
        <div class="card hide">
          <div class="issue-heading">
            <img
              class="issue-type-icon"
              @error=${(x) => {
                x.issueData! = { ...x.issueData!, issue_type_icon: svgJira };
              }}
              src="${(x) => x.issueData!.issue_type_icon}"
              alt="${(x) => x.issueData!.issue_type}"
            />
            <div class="issue-summary">
              <span class="issue-key">${(x) => x.issueData!.key}</span>
              <span class="issue-title">${(x) => x.issueData!.title}</span>
            </div>
          </div>
          <div class="issue-meta">
            <span class="meta-item">
              <img
                class="meta-icon"
                src="${(x) => x.issueData!.reporter_icon}"
                alt="Reporter: ${(x) => x.issueData!.reporter}"
              />
              ${(x) => x.issueData!.reporter}
            </span>
            <span
              class="status-pill"
              style="background-color: ${(x) => x.issueData!.status_color}; color: ${(x) => statusTextColor(x.issueData!.status_color)}"
            >
              ${(x) => x.issueData!.status}
            </span>
            <span class="meta-item">
              <img
                class="meta-icon"
                @error=${(x) => {
                  x.issueData! = { ...x.issueData!, priority_icon: svgJira };
                }}
                src="${(x) => x.issueData!.priority_icon}"
                alt="Priority: ${(x) => x.issueData!.priority}"
              />
              ${(x) => x.issueData!.priority}
            </span>
          </div>
          <dl class="fields">
            ${repeat(
              (x) => Object.entries(x.issueData!.fields),
              html`
                <dt>${(x) => x[0]}</dt>
                <dd>${(x) => x[1]}</dd>
              `,
            )}
          </dl>
          <a
            class="message-link button-link"
            href="${(x) => x.issueData!.url}"
            target="_blank"
            rel="noopener noreferrer"
            >View issue</a
          >
        </div>
      `,
    )}
  </template>
`;

export function register(o: string, a: string) {
  scriptOrigin = o;
  accountUUID = a;
  OmnitarIssueElement.define({
    name: "omnitar-issue",
    template: issueTemplateHTML,
    styles: [cssGlobal, cssNoProfile],
  });
}

export default register;
