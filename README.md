# SlackAtar

## Deploy to DigitalOcean
[![Deploy to DO](https://www.digitalocean.com/api/static-content/v1/images?src=https%3A%2F%2Fwww.deploytodo.com%2Fdo-btn-blue.svg&token=17b950890e7051c2a16c7739bad4f6d69e7773367b41d7b10e0e33a42a3e75b4)](https://cloud.digitalocean.com/apps/new?repo=https://github.com/halkeye/internal-profile-http/tree/main&refcode=7d6859326b6a)

A Gravatar-compatible profile/avatar service backed by Slack workspace directory, 
plus an embeddable `<omnitar-card>` hover-card web component.

On startup (and every `REFRESH_INTERVAL`, default 2h), it fetches every
Slack member with an email on file via `users.list` and indexes them by the
MD5 and SHA256 hash of their normalized email address — the same hashing
Gravatar uses — so any tool that already knows how to build a Gravatar URL
from an email can point at this service instead.

## Configuration

| Env var            | Required | Default | Description                                             |
|--------------------|----------|---------|-----------------------------------------------------------|
| `SLACK_BOT_TOKENS`  | yes      | —       | A CSV of slack bot tokens with `users:read`, `users:read.email`, and `users.profile:read` in the format of `TEAM_ID:BOT_TOKEN` |
| `PORT`             | no       | `8080`  | HTTP listen port.                                        |
| `STARTUP_TIMEOUT`  | no       | `2m`    | How long to retry the initial Slack load before exiting. |
| `LOG_LEVEL`        | no       | `info`  | logrus level (`debug`, `info`, `warn`, `error`).         |
| `APP_ENV`          | no       | `development` | Anything non development will disable vite passthrough and serve content via static/ directory |

## Endpoints

The server resolves its own Slack workspace/team ID at startup (via
`auth.test`) and scopes the profile/avatar routes under it, so URLs are
self-describing about which Slack org they came from:

- `GET /metrics` — Prometheus metrics.
- `GET /healthz` — liveness check.
- `GET /slack/{slackOrgId}/profiles/{profileIdentifier}` — CORS-enabled.
  `{profileIdentifier}` is the MD5 or SHA256 hash of a normalized
  (trimmed, lowercased) email. Returns `{"email", "name", "avatarUrl"}` JSON,
  or `404` if unknown. Requests with a `slackOrgId` other than this
  instance's also `404`.
- `GET /slack/{slackOrgId}/avatar/{profileIdentifier}` — CORS-enabled.
  Known identifier → `302` redirect to the person's Slack avatar URL.
  Unknown/no avatar → honors Gravatar's `?d=` convention: `d=404` returns
  `404`, anything else (including unset) serves a bundled default
  silhouette SVG.
- `GET /slack/{slackOrgId}/webcomponent.js` (or `/webcomponent.js`) — the `<omnitar-card>` custom element, rendered once
  at startup with this instance's `slackOrgId` baked in.

## Web component

```html
<script type="module" src="https://omnitar.apps.g4v.dev/slack/E0AFSMB25HU/webcomponent.js"></script>
<omnitar-card email="slack@gavinmogan.com">username or other display if nothing is found</omnitar-card>
```

Wraps any content; on hover or focus it shows a small card (avatar, name,
email) fetched from this same server's `/profiles` and `/avatar` endpoints.
No build step — plain JS, Shadow DOM for style isolation.

### Demo

http://omnitar.apps.g4v.dev/ (Assuming my slack dev sandbox is still running)

## Development

```bash
go test ./...
SLACK_BOT_TOKENS=E0AFSMB25HU:xoxb-... go run ./cmd/profile-http
```
