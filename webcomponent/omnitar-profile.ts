// <omnitar-profile> web component
//
// Usage:
//   <omnitar-profile email="slack@gavinmogan.com">Gavin Mogan</omnitar-card>
//

import { css, html, LitElement, type PropertyValues } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { map } from "lit/directives/map.js";
import { live } from "lit/directives/live.js";
import cachedFetch from "./util-cached-fetch.ts";
import hoverintent from "hoverintent";
import svgSlack from "./assets/slack.svg";

let scriptOrigin = "";
let accountUUID = "";

function normalizeEmail(email: string) {
  return email.trim().toLowerCase();
}

export const csvConverter = {
  fromAttribute(value: string | null): string[] {
    if (!value) return [];
    return value.split(",").map((item) => item.trim());
  },
  toAttribute(value: string[] | null): string | null {
    if (!value || !Array.isArray(value)) return null;
    return value.join(",");
  },
};
class Profile {
  id: string = "";
  team_id: string = "";
  name: string = "";
  email: string = "";
  avatar_url: string = "";
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

  .hide {
    display: none !important;
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
    display: grid;
    grid-template-columns: 48px minmax(140px, 1fr);
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
    width: 48px;
    height: 48px;
    border-radius: 50%;
    object-fit: cover;
    background: #eee;
    border: 1px solid #eee;
  }

  .card .name {
    font-weight: 600;
    font-size: 14px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .card .email {
    color: #6b7280;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-bottom: 4px;
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
    color: #1b1b1b;
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

@customElement("omnitar-profile")
export class OmnitarProfileElement extends LitElement {
  static styles = styles;

  @property({ type: String })
  source: string = "auto";

  @property({ type: String })
  email?: string;

  @property({ converter: csvConverter })
  fields: Array<string> = [];

  @property({ type: Boolean, reflect: true })
  visible = false;

  /** @internal */
  @state()
  profileData: Profile | null = null;

  /** @internal */
  private _hoverIntent: ReturnType<typeof hoverintent> | null = null;

  /** @internal */
  private _fetch() {
    if (!this.email) {
      return;
    }

    const email = normalizeEmail(this.email);
    let url = `${scriptOrigin}/account/${encodeURI(accountUUID)}/profiles/${encodeURI(this.source)}/${encodeURI(email)}`;
    if (this.fields.length != 0) {
      url += "?fields=" + encodeURI(this.fields.join("."));
    }

    cachedFetch(url).then((profileData) => {
      if (profileData && this.fields.length > 0) {
        ["email", "name"].forEach((field) => {
          if (!this.fields.includes(field)) {
            delete profileData[field];
          }
        });
        Object.keys(profileData.fields ?? {}).forEach((field) => {
          if (!this.fields.includes(field)) {
            delete profileData.fields[field];
          }
        });
      }
      this.profileData = profileData;
    });
  }

  willUpdate(changed: PropertyValues<this>) {
    const rawFields = this.fields as unknown;
    if (!Array.isArray(rawFields)) {
      this.fields = csvConverter.fromAttribute(String(rawFields ?? ""));
    }
    if (changed.has("email") || changed.has("fields")) {
      if (!this.email) {
        this.profileData = null;
        return;
      }
      this._fetch();
    }
  }

  updated(changed: PropertyValues<this>) {
    if (changed.has("profileData")) {
      this.toggleAttribute("has-data", !!this.profileData);
    }
  }

  connectedCallback() {
    super.connectedCallback();
    this._hoverIntent = hoverintent(this, this._show, this._hide);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._hide();
    this._hoverIntent?.remove();
  }

  /** @internal */
  private _show = () => {
    this.visible = true;
  };

  /** @internal */
  private _hide = () => {
    this.visible = false;
  };

  render() {
    if (!this.profileData) {
      return html`<slot></slot>`;
    }
    const profile = this.profileData;
    return html`
      <span class="display-name">
        <img class="icon" src=${live(svgSlack)} alt="Slack" />
        ${profile.name}
      </span>
      <div class="card ${this.visible ? "" : "hide"}">
        <div style="height: 100%">
          <img alt="profile photo" class="avatar" src="${profile.avatar_url}" />
        </div>
        <div>
          <div class="name">${profile.name}</div>
          <div class="email">${profile.email}</div>
          <dl class="fields">
            ${map(
              Object.entries(profile.fields ?? {}),
              ([key, value]) => html`
                <dt>${key}</dt>
                <dd>${value}</dd>
              `,
            )}
          </dl>
          <a
            class="message-link button-link"
            href="https://app.slack.com/client/${profile.team_id}/${profile.id}"
            target="_blank"
            >Message</a
          >
        </div>
      </div>
    `;
  }
}

export function register(o: string, a: string) {
  scriptOrigin = o;
  accountUUID = a;
}

export default register;
