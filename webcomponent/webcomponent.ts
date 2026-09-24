// <omnitar-card> web component
//
// Usage:
//   <script type="module" src="https://omnitar.apps.g4v.dev/slack/E0AFSMB25HU/webcomponent.js"></script>
//   <omnitar-card email="slack@gavinmogan.com">Gavin Mogan</omnitar-card>
//
// Wraps its content; on hover/focus it shows a small card with the
// person's Slack avatar, name, and email, fetched from this same origin's
// /slack/$ORG_ID/profiles/{hash} and /slack/$ORG_ID/avatar/{hash}
// endpoints (hash = SHA256 of the normalized email).
//
//

import templateHTML from "./webcomponent.html?raw";
import hoverIntent from "hoverintent";

const SCRIPT_ORIGIN = new URL(import.meta.url).origin;
const ACCOUNT_UUID =
  new URL(import.meta.url).pathname.match("/account/([\\w-]+)/")?.[1] ??
  new URLSearchParams(new URL(import.meta.url).search).get("accountUUID") ??
  "";

let templateElm = document.createElement("template");
templateElm.innerHTML = templateHTML;

function normalizeEmail(email: string) {
  return email.trim().toLowerCase();
}

// async function sha256Hex(input: string) {
//   const data = new TextEncoder().encode(input);
//   const digest = await crypto.subtle.digest("SHA-256", data);
//   return Array.from(new Uint8Array(digest))
//     .map((b) => b.toString(16).padStart(2, "0"))
//     .join("");
// }

const cache = new Map<string, Promise<Profile | null>>();
const cachedFetch = async (url: string) => {
  if (!cache.has(url)) {
    cache.set(
      url,
      (async (url) => {
        const res = await fetch(url);
        if (res.ok) {
          const profile = await res.json();
          return profile;
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

class OmnitarProfileElement extends HTMLElement {
  private _email: string = "";
  private _profile: Profile | null = null;
  private _hoverIntent: any;

  static observedAttributes = ["email"];

  attributeChangedCallback(name: string, _oldValue: string, newValue: string) {
    if (name === "email") {
      newValue = normalizeEmail(newValue);
      if (newValue != this._email) {
        cachedFetch(
          `${SCRIPT_ORIGIN}/account/${encodeURI(ACCOUNT_UUID)}/profiles/${encodeURI(newValue)}`,
        ).then((profile) => {
          this._handleSlackProfileCardProfile(profile);
        });
      }
      this._email = newValue;
      if (this.shadowRoot) {
        this.shadowRoot.innerHTML = "";
        this.shadowRoot.appendChild(templateElm.content.cloneNode(true));
      }
    }
  }

  private _handleSlackProfileCardProfile = (profile: null | Profile) => {
    this._profile = profile;
    if (!this._profile) {
      return;
    }

    if (this.shadowRoot) {
      this.shadowRoot
        .querySelector(".display-name slot")!
        .classList.add("hide");
      const displayName = this.shadowRoot.querySelector(".display-name span")!;
      displayName.classList.remove("hide");
      displayName.textContent = "🪪 " + this._profile.name.toString();

      this.shadowRoot.querySelector(".name")!.textContent =
        this._profile?.name ?? "";
      this.shadowRoot.querySelector(".email")!.textContent =
        this._profile?.email ?? "";
      this.shadowRoot
        .querySelector(".avatar")
        ?.setAttribute("src", this._profile?.avatar_url ?? "");
      this.shadowRoot
        .querySelector(".message-link")
        ?.setAttribute(
          "href",
          `https://app.slack.com/client/${this._profile?.team_id}/${this._profile?.id}`,
        );
      const fieldsElm = this.shadowRoot.querySelector(".fields");
      if (fieldsElm) {
        fieldsElm.innerHTML = "";
        for (const [key, value] of Object.entries(this._profile.fields ?? {})) {
          const title = document.createElement("dt");
          title.textContent = key;
          fieldsElm.appendChild(title);

          const valueElm = document.createElement("dd");
          valueElm.textContent = value;
          fieldsElm.appendChild(valueElm);
        }
      }
    }
  };

  constructor() {
    super();
    let shadowRoot = this.attachShadow({ mode: "open" });
    shadowRoot.appendChild(templateElm.content.cloneNode(true));
  }

  connectedCallback() {
    if (this.shadowRoot) {
      this.shadowRoot
        .querySelector(".display-name slot")!
        .classList.remove("hide");
    }
    this._hoverIntent = hoverIntent(this, this._show, this._hide);
  }

  disconnectedCallback() {
    this._hide();
    this._hoverIntent?.remove();
  }

  _show = async () => {
    if (!this._profile) {
      return;
    }
    if (this.shadowRoot) {
      this.shadowRoot.querySelector(".card")?.classList.remove("hide");
    }
  };

  _hide = () => {
    if (!this._profile) {
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

declare global {
  interface HTMLElementTagNameMap {
    "omnitar-profile": OmnitarProfileElement;
  }
}

window.customElements.define("omnitar-profile", OmnitarProfileElement);
