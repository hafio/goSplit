# GoSplit

Self-hosted expense splitting for households, trips and groups of friends. GoSplit is a
Go rebuild of [SplitPro](https://github.com/oss-apps/split-pro), the open-source Splitwise
alternative, with feature parity except OAuth/OIDC sign-in (GoSplit uses magic links and
passwords).

One static binary. SQLite by default, PostgreSQL when you need it. Server-rendered pages
that work with JavaScript off, installable as a PWA. No external services required.

## Features

- Friends and groups, with join links and per-currency balances that are always computed,
  never stored
- Five split methods (equal, percentage, exact, shares, adjustment), categories, notes,
  soft delete, and edits that refuse to overwrite someone else's concurrent change
- Settlements with a friend or inside a group, or settle a whole group in one action
- Filterable activity feed; move expenses between groups; archive groups; collapse old
  history into one balance-preserving entry
- Currency conversion with live rates, recurring expenses on a cron rule
- In-app notifications with a bell and unread count, plus opt-in Web Push and email,
  nine languages, six colour themes with light and dark
- Import friends and groups from Splitwise; optional bank sync through Plaid
- Encrypted whole-instance backups and restores, from the CLI, the admin page, a schedule
  or a watched directory
- Admin console to create, edit, deactivate and sign in users

Details in [docs/features.md](docs/features.md).

## Quick start

You need a `SESSION_SECRET` of at least 16 characters. Generate one with
`openssl rand -base64 32`, then **store it and reuse the same value on every start**: it
derives the encryption key for backup archives, and an archive made under one secret can
never be restored under another.

### Docker Compose (SQLite)

```sh
git clone https://github.com/hafio/gosplit.git && cd gosplit
export SESSION_SECRET=$(openssl rand -base64 32)
docker compose up -d --build
# open http://localhost:8080
```

This builds the image from your checkout and keeps data in the `gosplit-data` volume.
Only the variables listed in the `environment:` block of `docker-compose.yml` reach the
container (`SESSION_SECRET`, `BASE_URL`, `ADMIN_EMAILS`, `BACKUP_DIR`); add others there.
For PostgreSQL, uncomment the `db` service and follow
[docs/deployment.md](docs/deployment.md#postgresql-with-compose).

### Docker, published image

```sh
docker run -d --name gosplit \
  -p 8080:8080 \
  -v gosplit-data:/data \
  -e SESSION_SECRET="$(openssl rand -base64 32)" \
  -e BACKUP_DIR=/data/backups \
  ghcr.io/hafio/gosplit:latest
```

Images are published for `linux/amd64` and `linux/arm64` with tags `X.Y.Z`, `X.Y`, `X` and
`latest`. The image keeps its database and uploads under `/data` and listens on `8080`.
Production flags, including keeping the secret out of `docker inspect`:
[Plain Docker](docs/deployment.md#plain-docker).

### From source

Requires Go 1.21 or newer on your PATH. `go.mod` pins `toolchain go1.27.0`, so an older Go
downloads and runs that toolchain automatically; Go 1.27 or newer builds with itself.

```sh
git clone https://github.com/hafio/gosplit.git && cd gosplit
cp .env.example .env          # set SESSION_SECRET; SQLite and log-only mail by default
set -a; . ./.env; set +a      # the binary reads the environment, not the file
go run ./cmd/gosplit
```

More, including the dev scripts: [Running locally](docs/development.md#running-locally).

### First run

Open the app and register with a name, email and password; registering signs you in
straight away. Put your own address in `ADMIN_EMAILS` before you register to become the
administrator, see [First admin](docs/deployment.md#first-admin). Without a mail server,
magic-link and password-reset emails are written to the log (`docker compose logs app`)
instead of being sent, so copy the link from there when you use those paths.

## Going to production

Each line links to the section that explains it.

- Put it behind HTTPS and set `BASE_URL` to the public `https://` address. That is what
  turns on secure cookies. [Reverse proxy](docs/deployment.md#reverse-proxy-and-https)
- Keep `/data` on a persistent volume and set `BACKUP_DIR` to a path on it; the image does
  not set one. [The container](docs/deployment.md#the-container)
- Configure SMTP so people other than you can sign in. [Mail](docs/deployment.md#mail)
- Turn on scheduled backups with `BACKUP_CRON` and copy the archives off the machine.
  [Backup and restore](docs/backup-restore.md)
- Use PostgreSQL for more than one replica. [Choosing an engine](docs/deployment.md#choosing-an-engine)
- Probe `GET /healthz` from outside; the image has no shell for an internal health check.
  [Health check](docs/deployment.md#health-check)
- Back up before every upgrade. [Upgrading](docs/deployment.md#upgrading)

## Configuration

Everything is an environment variable. The ones almost everyone sets:

| Variable | Purpose |
| --- | --- |
| `SESSION_SECRET` | required; 16+ characters; derives the encryption key for backups |
| `BASE_URL` | public URL; `https://` enables secure cookies |
| `DATABASE_URL` | `file:./data/gosplit.db` (default) or a PostgreSQL URL / DSN |
| `EMAIL_SERVER_HOST`, `EMAIL_SERVER_PORT`, `EMAIL_SERVER_USER`, `EMAIL_SERVER_PASSWORD`, `FROM_EMAIL` | outgoing mail |
| `ADMIN_EMAILS` | comma-separated addresses that become admins |
| `BACKUP_DIR` | where archives go; set it to `/data/backups` in a container |

The full reference, with defaults and behaviour, is
[docs/configuration.md](docs/configuration.md). `.env.example` is a working local
template.

## Command line

| Command | What it does |
| --- | --- |
| `gosplit` | start the server |
| `gosplit version` | print the build version |
| `gosplit backup [-o DIR]` | write an encrypted archive of everything |
| `gosplit inspect ARCHIVE` | show an archive's header; needs no secret or database |
| `gosplit restore --file ARCHIVE` | validate an archive against this database; the database is unchanged |
| `gosplit restore --file ARCHIVE --force` | replace all data with the archive |

There is no top-level help; an unrecognised first argument starts the server. In the
container, run `docker exec <container> /app/gosplit ...`. Details, including the
`--no-pre-dump` flag and what happens during a restore, are in
[docs/backup-restore.md](docs/backup-restore.md).

## Documentation

- [docs/features.md](docs/features.md) -- what the app does, screen by screen
- [docs/configuration.md](docs/configuration.md) -- every setting
- [docs/deployment.md](docs/deployment.md) -- running it for real
- [docs/backup-restore.md](docs/backup-restore.md) -- archives, restores, disaster recovery
- [docs/architecture.md](docs/architecture.md) -- how it is built
- [docs/development.md](docs/development.md) -- building, testing, releasing, extending

## Development

`./scripts/dev.sh all` (or `.\scripts\dev.ps1 all`) builds, vets and tests; `full` adds
coverage, the image, vulnerability scans and the knowledge graph. See
[docs/development.md](docs/development.md).

## License

GPL-3.0. See [LICENSE](LICENSE).
