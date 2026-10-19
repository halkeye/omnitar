# Omnitar

## Deploy to DigitalOcean
[![Deploy to DO](https://www.deploytodo.com/do-btn-blue.svg)](https://cloud.digitalocean.com/apps/new?repo=https://github.com/halkeye/omnitar/tree/main&refcode=7d6859326b6a)

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

### Demo

http://omnitar.apps.g4v.dev/

## Development

```bash
go test ./...
go run ./cmd/server
```
