// <omnitar-profile> web component
//
// Usage:
//   <omnitar-profile email="slack@gavinmogan.com">Gavin Mogan</omnitar-card>
//

import { css, html, LitElement, nothing, type PropertyValues } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
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

  .display-name {
    anchor-name: --omnitar-profile-trigger;
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
    position-anchor: --omnitar-profile-trigger;
    top: calc(anchor(bottom) + 8px);
    left: anchor(left);
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

/**
 * Hover card for a Slack user, looked up by email.
 */
@customElement("omnitar-profile")
export class OmnitarProfileElement extends LitElement {
  static styles = styles;

  /** Not yet working, but will be used to support multiple issue sources in the future. */
  @property({ type: String })
  source: string = "auto";

  /** Email of the profile you want to look up */
  @property({ type: String })
  email?: string;

  /** Limit to certain profile fields. If empty, all fields will be returned. */
  @property({ converter: csvConverter })
  fields: Array<string> = [];

  /** When set, keeps the card open. When unset, the card opens on hover only. */
  @property({ type: Boolean, reflect: true })
  visible = false;

  /** @internal */
  @state()
  profileData: Profile | null = null;

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
    if (
      this.profileData &&
      (changed.has("profileData") ||
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
    if (OmnitarProfileElement._anchorPositioningSupported) {
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
    if (!this.card || !this.profileData) {
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
    if (!this.profileData) {
      return html`<slot></slot>`;
    }
    const profile = this.profileData;
    return html`
      <span class="display-name">
        <img class="icon" src=${live(svgSlack)} alt="Slack" />
        ${profile.name ? profile.name : html`<slot></slot>`}
      </span>
      <div class="card ${this._cardOpen ? "" : "hide"}" popover="manual">
        <div style="height: 100%">
          <img alt="profile photo" class="avatar" src="${profile.avatar_url}" />
        </div>
        <div>
          ${profile.name
            ? html`<div class="name">${profile.name}</div>`
            : nothing}
          ${profile.email
            ? html`<div class="email">${profile.email}</div>`
            : nothing}
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
