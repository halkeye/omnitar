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
  }

  .display-name img {
    height: 1em;
    weidth: 1em;
  }

  .card {
    position: absolute;
    top: 75%;
    left: 0;
    margin-top: 8px;
    display: grid;
    align-items: start;
    gap: 10px;
    padding: 12px 14px;
    width: max-content;
    min-width: 220px;
    max-width: 500px;
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
    display: grid !important;
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

  .card img {
    height: 2em;
    weidth: 2em;
    vertical-align: middle;
  }

  .status-pill {
    background-color: #ddd;
    border: none;
    color: black;
    padding: 10px 20px;
    text-align: center;
    text-decoration: none;
    display: inline-block;
    margin: 4px 2px;
    cursor: pointer;
    border-radius: 16px;
  }

  .card dl.fields {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 2px 6px;
    margin: 6px 0 8px;
    font-size: 12px;
    color: #444;
  }

  .card dl.fields dt {
    font-weight: 500;
    color: #888;
  }

  .card dl.fields dd {
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .button-link {
    display: inline-block;
    padding: 6px 14px;
    font-size: 12px;
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
`;

const issueTemplateHTML = html<OmnitarIssueElement>`
  <template>
    ${when(
      (x) => !x.issueData,
      html<OmnitarIssueElement>`<slot></slot>`,
      html<OmnitarIssueElement>`
        <span class="display-name"
          ><span
            ><img class="icon" src=${svgJira} alt="jira icon" /> ${(x) =>
              x.issueData!.key}</span
          ></span
        >
        <div class="card hide">
          <div>
            <div>
              <img
                @error=${(x) => {
                  x.issueData! = { ...x.issueData!, issue_type_icon: svgJira };
                }}
                src="${(x) => x.issueData!.issue_type_icon}"
                alt="${(x) => x.issueData!.issue_type} icon"
              />
              ${(x) => x.issueData!.key}: ${(x) => x.issueData!.title}
            </div>
            <div>
              <img
                src="${(x) => x.issueData!.reporter_icon}"
                alt="reporter(${(x) => x.issueData!.reporter}) icon"
              />
              <div
                class="status-pill"
                style="background-color: ${(x) => x.issueData!.status_color}"
              >
                ${(x) => x.issueData!.status}
              </div>
              <img
                @error=${(x) => {
                  x.issueData! = { ...x.issueData!, priority_icon: svgJira };
                }}
                src="${(x) => x.issueData!.priority_icon}"
                alt="priority (${(x) => x.issueData!.priority}) icon"
              />
              ${(x) => x.issueData!.priority}
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
              >View</a
            >
          </div>
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
