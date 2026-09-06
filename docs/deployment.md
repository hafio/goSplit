# Deployment

Running GoSplit for real: the container contract, HTTPS, PostgreSQL, mail, upgrades and
what to monitor. For operators. The [README](../README.md#quick-start) has the
copy-paste quick starts; this page is what to read before exposing an instance to other
people.

## The container

The release image is `ghcr.io/hafio/gosplit`, built for `linux/amd64` and `linux/arm64`,
with a provenance attestation and an SBOM attached.

| Tag | Meaning |
| --- | --- |
| `1.2.3` | one release, never moves |
| `1.2` | latest patch of that minor |
| `1` | latest release of that major |
| `latest` | newest release |

Release notes carry the image digest. Pin by digest (`ghcr.io/hafio/gosplit@sha256:...`)
if you need a deployment that cannot change under you.

What is inside:

- One static binary at `/app/gosplit`, running as the distroless `nonroot` user. There is
  no shell, no package manager and no `curl`.
- `VOLUME /data`. The image sets `DATABASE_URL=file:/data/gosplit.db` and
  `UPLOAD_DIR=/data/uploads`, so with SQLite everything that matters lives on that one
  volume. Back it up, or better, use the built-in [backups](backup-restore.md).
- `BACKUP_DIR` is **not** set in the image. Its default, `./data/backups`, resolves against
  the `/app` working directory: off the volume and not writable by `nonroot`, so every
  backup would fail. Pass `BACKUP_DIR=/data/backups` (the Compose file does).
- Listens on `:8080` (`ADDR`). `EXPOSE 8080`.
- `SESSION_SECRET` is deliberately not set in the image. The container exits with
  `config: SESSION_SECRET is required` until you provide one.

Because there is no shell, run the CLI with `docker exec <container> /app/gosplit ...`.

## Reverse proxy and HTTPS

Run GoSplit behind a TLS-terminating proxy (Caddy, nginx, Traefik, a cloud load
balancer) and set `BASE_URL` to the public `https://` address. That prefix is what turns
on the `Secure` flag for session and CSRF cookies, and it is the host used in emailed
links. An instance on `http://` with real users leaks session cookies to anyone on the
path.

Paths the proxy must pass through unchanged, besides the app pages:

| Path | Why |
| --- | --- |
| `/healthz` | health probe, see below |
| `/static/*` | CSS, JavaScript, icons (long-cached, content-hashed) |
| `/uploads/*` | profile avatars |
| `/manifest.webmanifest`, `/sw.js`, `/offline` | the installable PWA and its offline page |

Note that `/uploads/*` is served without authentication, with a one-hour cache header, by
a plain file server with directory listing on: an anonymous `GET /uploads/` lists every
avatar filename. The names are random, but if you do not want them enumerable, block the
bare `/uploads/` path at the proxy. Avatars are the only thing stored there; do not put
anything else in `UPLOAD_DIR`.

Responses are gzip-compressed by the app for HTML, CSS, JavaScript, JSON and text, so the
proxy need not compress again. Request bodies are never decompressed; do not send
compressed uploads.

## Health check

`GET /healthz` is public. It pings the database and returns

```json
{"status":"ok","version":"v1.2.3"}
```

with `200`, or `503` when the database is unreachable. The version is the release tag, so
this is also how you check which build a container is running.

The image has no shell or `curl`, so a Docker `HEALTHCHECK` cannot be run inside it and
none is declared. Probe from outside: a Compose healthcheck on a sidecar, an orchestrator
HTTP probe, or your proxy's upstream check.

## Choosing an engine

**SQLite** is the default and the right choice for one container serving a household or
a small group. One file, no second service, and the built-in backups cover it. It runs
on a single connection, so a long CLI restore blocks the site for its duration.

**PostgreSQL** is for more than one replica, or when you already run it. Every replica
can run the scheduler (recurring expenses, session and cache cleanup, scheduled
backups); database leader locks keep each job, and startup auto-restore, to one replica
at a time. Point all replicas at the same `UPLOAD_DIR` (a shared volume) or avatars will
differ per replica.

Moving between engines is a backup on one and a restore on the other. See
[backup-restore.md](backup-restore.md).

## Docker Compose

The repository's `docker-compose.yml` runs SQLite on a named volume. It uses `build: .`,
so it compiles the app from your checkout rather than pulling the published image, and the
version it reports ends in `-dirty` because the build context is trimmed. Upgrading such a
deployment is `git pull` followed by the same command.

Only the variables in the file's `environment:` block reach the container:
`SESSION_SECRET` (required), `BASE_URL` and `ADMIN_EMAILS` (from your shell or from a
`.env` file next to the compose file, which Compose uses for substitution only), and
`BACKUP_DIR=/data/backups`. Add anything else, SMTP for instance, to that block.

```sh
export SESSION_SECRET=$(openssl rand -base64 32)   # once; keep it
docker compose up -d --build
```

To add scheduled backups, uncomment `BACKUP_CRON` and `BACKUP_RETENTION_COUNT` in the
file. For unattended restores, give the container a second directory as
`AUTO_RESTORE_DIR`; it must not be `BACKUP_DIR`, see
[restoring at startup](backup-restore.md#restoring-at-startup). Both directories are
under `/data` in the example so they sit on the persistent volume.

### PostgreSQL with Compose

Uncomment the `db` service, its volume, and the `depends_on` line, then point the app at
it:

```yaml
environment:
  SESSION_SECRET: "${SESSION_SECRET:?generate with: openssl rand -base64 32}"
  DATABASE_URL: "host=db port=5432 user=gosplit password=gosplit dbname=gosplit sslmode=disable"
```

Change the password in both places. The keyword DSN form needs no URL-encoding; the
`postgres://` URL form also works. See
[configuration.md](configuration.md#database) for both.

Data lives in the `gosplit-pg` volume; `/data` on the app container then holds only
avatars and backup archives.

## Plain Docker

```sh
docker volume create gosplit-data
docker run -d --name gosplit \
  -p 8080:8080 \
  -v gosplit-data:/data \
  -e SESSION_SECRET="$(openssl rand -base64 32)" \
  -e BASE_URL=https://split.example.com \
  -e BACKUP_DIR=/data/backups \
  --restart unless-stopped \
  ghcr.io/hafio/gosplit:latest
```

Pin a version tag (`0.4.1`, or `0` for the current major line) once you know which one you
run. Add `-e` flags for SMTP, `ADMIN_EMAILS` and anything else from
[configuration.md](configuration.md). To keep the secret out of your shell history and
the host process list, use `--env-file` with a root-only file instead of `-e`, and keep
that file: the same secret must be passed on every start. Either way the value is visible
in `docker inspect`, so treat access to the Docker socket as access to the secret.

## Mail

Without `EMAIL_SERVER_HOST`, sign-in links are printed to the container log. That works
for trying things out and for a single admin who can read the log, and for nobody else.
Set the SMTP variables before inviting anyone. Password sign-in works regardless, so
users who set a password are not blocked by a mail outage.

## First admin

Put your address in `ADMIN_EMAILS` before you register. Your account is promoted the
first time you sign in, and stays admin. Admins get `/admin` (user management) and
`/admin/backup`. Once you have an admin, consider `DISABLE_EMAIL_SIGNUP=true` and create
further users from the admin console.

## Upgrading

1. Take a backup: `docker exec gosplit /app/gosplit backup -o /data/backups`, or download
   one from `/admin/backup`. Copy it somewhere off the volume.
2. Pull the new image and recreate the container (with Compose: `git pull`, then
   `docker compose up -d --build`). Migrations run on start.
3. Check `/healthz` shows the new version and the footer of any page shows it too.

If you need to roll back, restore the archive with the **old** image, then stay on it.
An archive restores only into a database whose migrations match exactly; see
[backup-restore.md](backup-restore.md#migration-sets-must-match-exactly).

## Monitoring and logs

Logs are JSON lines on stdout, one object per line, at `LOG_LEVEL` (`info` by default).
Startup logs the version, the engine and the listen address. Any fatal condition ends
with one record whose `msg` is `fatal` and whose `err` names the cause and the fix, then
the process exits `1`.

HTML responses carry a `Server-Timing: render;dur=<ms>` header with the server-side render
time, if you want to graph it. The sign-in, register, forgot and reset pages and the
generic message page omit it on purpose.

Things worth alerting on:

- the container restarting in a loop (a bad archive in `AUTO_RESTORE_DIR`, or a
  configuration error; the last log line, the `fatal` record, says which);
- `/healthz` returning `503` (database unreachable);
- no new file in `BACKUP_DIR` for longer than your `BACKUP_CRON` interval.

## Security notes

- `SESSION_SECRET` is the key to every backup archive. Store it in a secret manager or a
  root-only env file and pass the same value on every start. Rotating it does not log
  anyone out (sessions are database rows); it does orphan every existing archive. Read
  [Archives are secrets](backup-restore.md#archives-are-secrets) first.
- Backup archives are as sensitive as the database. Restrict who can read `BACKUP_DIR`
  and where you copy them.
- Passwords are hashed with argon2id. Sessions are server-side; changing a password or
  an admin resetting one revokes that user's sessions.
- The Splitwise import takes the user's own API key on the page and does not store it.
- Bank sync (Plaid) is off until `PLAID_CLIENT_ID` and `PLAID_SECRET` are set. Note
  `PLAID_ENVIRONMENT` defaults to `sandbox`.

## See also

- [configuration.md](configuration.md) -- every setting
- [backup-restore.md](backup-restore.md) -- backups, restores, disaster recovery
- [development.md](development.md#versioning-and-release) -- how releases and image tags are produced
