# cronguard

[![CI](https://github.com/narrowcastdev/cronguard/actions/workflows/ci.yml/badge.svg)](https://github.com/narrowcastdev/cronguard/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/narrowcastdev/cronguard)](https://goreportcard.com/report/github.com/narrowcastdev/cronguard)

Self-hosted cron job monitor. Single binary. No dependencies.

Your cron jobs ping cronguard over HTTP. If a ping doesn't arrive on schedule, you get alerted via webhook or email. Dead man's switch pattern — if it stops hearing from your jobs, something is wrong.

**Built for self-hosters and homelab operators** running 10-50 cron jobs who want to know when something stops running.

## Quick Start

```bash
# Download the latest release, then:
./cronguard
# Open http://localhost:8099
```

1. Create a check with a name and cron schedule (e.g. `0 3 * * *` for daily at 3 AM)
2. Add the ping URL to your cron job:

```bash
# Report success after your job runs
0 3 * * * /usr/local/bin/backup.sh && curl -fsS http://localhost:8099/ping/nightly-backup

# Or capture output
0 3 * * * /usr/local/bin/backup.sh 2>&1 | curl -fsS -d @- http://localhost:8099/ping/nightly-backup

# Or report failures
0 3 * * * /usr/local/bin/backup.sh || curl -fsS -X POST http://localhost:8099/ping/nightly-backup/fail
```

3. Add alert destinations (webhook URL or email) to get notified when a job misses its schedule

## Installation

**Binary** — download from [GitHub Releases](https://github.com/narrowcastdev/cronguard/releases):

```bash
# Linux (amd64)
curl -Lo cronguard.tar.gz https://github.com/narrowcastdev/cronguard/releases/latest/download/cronguard_linux_amd64.tar.gz
tar xzf cronguard.tar.gz
sudo mv cronguard /usr/local/bin/
```

**Docker:**

```bash
docker run -d \
  -p 127.0.0.1:8099:8099 \
  -v cronguard-data:/data \
  -e CRONGUARD_PASSWORD=secret \
  narrowcastdev/cronguard
```

**Docker Compose:**

```yaml
services:
  cronguard:
    image: narrowcastdev/cronguard:latest
    container_name: cronguard
    restart: unless-stopped
    ports:
      - "127.0.0.1:8099:8099"
    volumes:
      - cronguard-data:/data
    env_file:
      - .env  # CRONGUARD_PASSWORD=your-secret-here

  # Example: a backup job that pings cronguard after each run
  backup:
    image: alpine:latest
    command: sh -c '/backup.sh && curl -fsS http://cronguard:8099/ping/nightly-backup'
    depends_on:
      - cronguard

volumes:
  cronguard-data:
```

**Go install:**

```bash
go install github.com/narrowcastdev/cronguard/cmd/cronguard@latest
```

**Build from source:**

```bash
git clone https://github.com/narrowcastdev/cronguard.git
cd cronguard
go build -o cronguard ./cmd/cronguard
```

## Configuration

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--listen` | - | auto | Address to bind (`127.0.0.1:8099` without password, `0.0.0.0:8099` with) |
| `--data` | - | `./cronguard.db` | Path to SQLite database file |
| `--password` | `CRONGUARD_PASSWORD` | - | Admin password for dashboard and API |

When no password is set, cronguard listens on localhost only (local-only mode, no auth required). Set a password to listen on all interfaces with basic auth protection. When prompted, leave the username empty and enter the password.

Ping endpoints (`/ping/*`) never require authentication regardless of mode.

## Alerting

### Webhook

Add a webhook URL as an alert destination. cronguard POSTs JSON when a check goes down or recovers:

```json
{
  "check": "nightly-backup",
  "status": "down",
  "last_ping": "2026-04-13T03:01:22Z",
  "expected": "2026-04-14T03:00:00Z",
  "message": "Check \"nightly-backup\" is down. Last ping was 26 hours ago."
}
```

Works with Slack incoming webhooks, Discord webhooks, ntfy, Gotify, or any HTTP endpoint.

### Email

Configure SMTP settings in the dashboard under Settings. cronguard sends plain text emails with the alert details. Gmail users: enable 2-Step Verification and use an [App Password](https://myaccount.google.com/apppasswords).

## How It Works

cronguard uses the **dead man's switch** pattern. Instead of actively checking if your services are up, it waits for your cron jobs to check in. A background loop runs every 30 seconds — for each check, it computes when the next ping should arrive based on the cron schedule and grace period. If a ping is overdue, it sends alerts.

| Status | Meaning |
|--------|---------|
| **new** | Check created, no pings received yet |
| **up** | Last ping received on schedule |
| **late** | Next ping is overdue but still within grace period |
| **down** | Ping is overdue past the grace period — alerts sent |

## What This Is NOT

- Not a replacement for Healthchecks.io (no team features, no API keys, no history)
- Not an uptime monitor (it doesn't make outbound HTTP checks)
- Not a metrics collector (no charts, no trends, no Prometheus)
- No network access, no telemetry, no external API calls — runs entirely on your machine

It's a single binary that tells you when your cron jobs stop running. That's it.

## Contributing

Contributions welcome — open an issue, send a PR, or fork it and make it your own.

```bash
git clone https://github.com/narrowcastdev/cronguard.git
cd cronguard
go build ./cmd/cronguard
go test ./...
```

## License

[MIT](LICENSE)
