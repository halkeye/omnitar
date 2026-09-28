const SCRIPT_ORIGIN = new URL(import.meta.url).origin;
const ACCOUNT_UUID =
  new URL(import.meta.url).pathname.match("/account/([\\w-]+)/")?.[1] ??
  new URLSearchParams(new URL(import.meta.url).search).get("accountUUID") ??
  "";

import registerIssue from "./omnitar-issue.ts";
import registerProfile from "./omnitar-profile.ts";
registerProfile(SCRIPT_ORIGIN, ACCOUNT_UUID);
registerIssue(SCRIPT_ORIGIN, ACCOUNT_UUID);
