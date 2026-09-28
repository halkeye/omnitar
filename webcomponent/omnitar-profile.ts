// <omnitar-profile> web component
//
// Usage:
//   <omnitar-profile email="slack@gavinmogan.com">Gavin Mogan</omnitar-card>
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

enableDebug();

let scriptOrigin = "";
let accountUUID = "";

function normalizeEmail(email: string) {
  return email.trim().toLowerCase();
}

class OmnitarProfileElement extends FASTElement {
  @attr
  source: string = "auto";
  @attr
  email?: string;
  @observable
  profileData: Profile | null = null;

  private _hoverIntent: ReturnType<typeof hoverIntent> | null = null;

  profileDataChanged(_: Profile, newValue?: Profile) {
    if (newValue) {
      this.$fastController.addStyles(cssHasProfile);
    } else {
      this.$fastController.addStyles(cssNoProfile);
    }
  }

  emailChanged(_: string, newValue?: string) {
    if (!newValue) {
      this.profileData = null;
      return;
    }

    newValue = normalizeEmail(newValue);
    cachedFetch(
      `${scriptOrigin}/account/${encodeURI(accountUUID)}/profiles/${encodeURI(this.source)}/${encodeURI(newValue)}`,
    ).then((profileData) => {
      this.profileData = profileData;
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
    if (!this.profileData) {
      return;
    }
    if (this.shadowRoot) {
      this.shadowRoot.querySelector(".card")?.classList.remove("hide");
    }
  };

  _hide = () => {
    if (!this.profileData) {
      return;
    }
    if (this.shadowRoot) {
      this.shadowRoot.querySelector(".card")?.classList.add("hide");
    }
  };
}

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

const profileTemplateHTML = html<OmnitarProfileElement>`
  <template>
    ${when(
      (x) => !x.profileData,
      html<OmnitarProfileElement>`<slot></slot>`,
      html<OmnitarProfileElement>`
        <span class="display-name">🪪 ${(x) => x.profileData!.name}</span>
        <div class="card hide">
          <div style="height: 100%">
            <img
              alt="profile photo"
              class="avatar"
              src="${(x) => x.profileData!.avatar_url}"
            />
          </div>
          <div>
            <div class="name">${(x) => x.profileData!.name}</div>
            <div class="email">${(x) => x.profileData!.email}</div>
            <dl class="fields">
              ${repeat(
                (x) => Object.entries(x.profileData!.fields),
                html`
                  <dt>${(x) => x[0]}</dt>
                  <dd>${(x) => x[1]}</dd>
                `,
              )}
            </dl>
            <a
              class="message-link button-link"
              href="${(x) =>
                `https://app.slack.com/client/${x.profileData!.team_id}/${x.profileData!.id}`},"
              target="_blank"
              >Message</a
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
  OmnitarProfileElement.define({
    name: "omnitar-profile",
    template: profileTemplateHTML,
    styles: [cssGlobal, cssNoProfile],
  });
}

export default register;
