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

All configuration is read from environment variables (see `internal/config/config.go`).

| Env var                   | Required | Default                | Description |
|---------------------------|----------|------------------------|-------------|
| `APP_ENV`                 | no       | `development`          | Anything other than `development` disables the vite passthrough and serves content from the embedded `static/` directory. The Docker image sets `production`. |
| `PORT`                    | no       | `8080`                 | HTTP listen port. |
| `LOG_LEVEL`               | no       | `info`                 | Log level (`debug`, `info`, `warn`, `error`). |
| `DATABASE_URL`            | no       | `sqlite://./db.sqlite` | Database for accounts, tokens and sessions. Supported schemes: `sqlite`, `postgres`/`postgresql`, `mysql`/`mariadb`. |
| `SESSION_KEY`             | in production | `omnitar-session-key` (development only) | Secret used to sign session cookies. The server refuses to start with the default key unless `APP_ENV=development`. Use a long random value. |
| `SLACK_CLIENT_ID`         | no       | -                      | Slack OAuth app client ID (needed for "Add Slack"). |
| `SLACK_CLIENT_SECRET`     | no       | -                      | Slack OAuth app client secret. |
| `ATLASSIAN_CLIENT_ID`     | no       | -                      | Atlassian OAuth app client ID (needed for "Add Atlassian"). |
| `ATLASSIAN_CLIENT_SECRET` | no       | -                      | Atlassian OAuth app client secret. |

The Slack and Atlassian variables are optional at startup, but the matching login
flow (`/auth/slack`, `/auth/atlassian`) won't work without them.

On DigitalOcean App Platform, `DATABASE_URL` is wired to the managed Postgres
database by `.do/deploy.template.yaml`.

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
