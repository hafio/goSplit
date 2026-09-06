# Configuration

Every setting GoSplit reads, what it defaults to, and what it changes. For operators; if
you only want to get running, the [README quick start](../README.md#quick-start) covers
the three values that matter.

## How settings are read

- Settings come from the **process environment only**. The binary never reads a `.env`
  file. Load one into your shell first (`set -a; . ./.env; set +a`), or pass variables
  through Docker, Compose, systemd, or your orchestrator.
- An **empty value counts as unset** and falls back to the default. You cannot clear a
  setting that has a non-empty default by writing `NAME=`.
- **Unparseable numbers, booleans and durations fall back silently** to the default.
  `SCHEDULER=yes` or `EMAIL_SERVER_PORT=smtp` boot normally on the default value, so check
  the startup log if a setting does not seem to take effect.
- Booleans accept what Go's `strconv.ParseBool` accepts: `true`, `false`, `1`, `0`, `t`,
  `f`, `T`, `F`, `TRUE`, `FALSE`.
- Durations use Go syntax: `720h`, `30m`, `1h30m`. Days are not a unit; write `24h`.
- A few settings are validated at startup and stop the boot with a message naming the
  fix. They are marked **fatal** below. Everything else is accepted as given.
- The container image sets `DATABASE_URL=file:/data/gosplit.db`,
  `UPLOAD_DIR=/data/uploads` and `ADDR=:8080` in the image itself. Inside a container
  those are the effective defaults, not the ones in the tables below. It does **not**
  set `BACKUP_DIR`, whose relative default would resolve under `/app`, outside the
  volume; set `BACKUP_DIR=/data/backups` yourself.
- Cron rules (`BACKUP_CRON`) are standard 5-field `min hour dom mon dow` and are evaluated
  in UTC regardless of the host's time zone.

## Core

| Variable | Default | Notes |
| --- | --- | --- |
| `SESSION_SECRET` | none | **Required, fatal if missing or shorter than 16 characters.** Derives the encryption key for every backup archive (scrypt, then HKDF). It is not used for sessions or CSRF: session cookies are random tokens stored in the database and the CSRF token is a random double-submit cookie. Generate with `openssl rand -base64 32` and keep the same value for the life of the instance. Rotating it makes every existing archive unrestorable and logs nobody out; see [backup-restore.md](backup-restore.md#archives-are-secrets). |
| `BASE_URL` | `http://localhost:8080` | The public URL, used in emailed links. An `https://` prefix is also what turns on the `Secure` flag for cookies, so set it correctly behind a TLS proxy. |
| `ADDR` | `:8080` | Listen address. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` (or `warning`), `error`. Any other value means `info`. Per-message SMTP sends are logged at `debug`; with no SMTP host the log-only mailer records every message, recipient, subject and body (links included) at `info`. Logs are JSON lines on stdout. |

## Database

| Variable | Default | Notes |
| --- | --- | --- |
| `DATABASE_URL` | `file:./data/gosplit.db` | Selects the engine as well as the location. See the forms below. An unrecognised form is **fatal**. |

Accepted forms:

| Form | Engine | Example |
| --- | --- | --- |
| `file:<path>` or `sqlite://<path>` | SQLite | `file:/data/gosplit.db` |
| `postgres://` or `postgresql://` URL | PostgreSQL | `postgres://gosplit:secret@db:5432/gosplit?sslmode=disable` |
| libpq keyword DSN | PostgreSQL | `host=db port=5432 user=gosplit password='p@ss w0rd' dbname=gosplit sslmode=disable` |

A bare path such as `./data/gosplit.db` is rejected; the `file:` prefix is required.
Prefer the keyword DSN for PostgreSQL when the password contains characters that a URL
would need percent-encoded. The keyword form is recognised by the presence of any libpq
keyword (`host=`, `dbname=`, `user=`, ...), and that check runs before the SQLite check.

Migrations run automatically when the server starts. SQLite uses a single connection;
PostgreSQL uses a pool and is the engine to choose when you run more than one replica.

## Uploads

| Variable | Default | Notes |
| --- | --- | --- |
| `UPLOAD_DIR` | `./data/uploads` | Where profile avatars are stored. Served publicly at `/uploads/`. Included in every backup archive, and must be set for a restore to work. |
| `UPLOAD_MAX_FILE_SIZE_MB` | `10` | Maximum avatar upload. Also the per-file cap a restore enforces on the uploads inside an archive. A backup includes every file regardless of size, so lowering this below a file already archived makes that archive fail validation until the value is raised again. |

## Email (SMTP)

| Variable | Default | Notes |
| --- | --- | --- |
| `EMAIL_SERVER_HOST` | none | Leave empty and GoSplit prints magic-link and password-reset links to the log instead of sending mail. Convenient for local use, unusable for real users. |
| `EMAIL_SERVER_PORT` | `587` | |
| `EMAIL_SERVER_USER` | none | SMTP username, if your relay needs one. |
| `EMAIL_SERVER_PASSWORD` | none | SMTP password. |
| `EMAIL_TLS_REJECT_UNAUTHORIZED` | `true` | Set `false` only for a relay with a self-signed certificate. |
| `FROM_EMAIL` | `no-reply@gosplit.local` | Sender address. Also the fallback contact for Web Push (below). |

## Web Push notifications

| Variable | Default | Notes |
| --- | --- | --- |
| `WEB_PUSH_PUBLIC_KEY` | none | VAPID public key. Push is enabled only when both keys are set. Otherwise the profile page still shows the notifications section, and enabling reports that push is not configured on this server. |
| `WEB_PUSH_PRIVATE_KEY` | none | VAPID private key. |
| `WEB_PUSH_EMAIL` | `FROM_EMAIL` | Contact address sent to push services as `mailto:`. Optional. |

Generate a VAPID key pair with any web-push tool, for example
`npx web-push generate-vapid-keys`.

## Currency rates

| Variable | Default | Notes |
| --- | --- | --- |
| `CURRENCY_RATE_PROVIDER` | `frankfurter` | `frankfurter` (ECB rates, no key), `openexchangerates` (alias `oxr`, needs the app id below), or `none` (alias `disabled`) to make no outbound rate calls. Any other value falls back to Frankfurter. |
| `OPEN_EXCHANGE_RATES_APP_ID` | none | Only used with `openexchangerates`. |

Fetched rates are cached in the database and pruned by the scheduler after
`CACHE_RETENTION_INTERVAL`.

## Bank sync (Plaid, optional)

| Variable | Default | Notes |
| --- | --- | --- |
| `PLAID_CLIENT_ID` | none | Bank sync is enabled only when both this and the secret are set. The `/bank` page still exists when unset but reports the feature as off. |
| `PLAID_SECRET` | none | |
| `PLAID_ENVIRONMENT` | `sandbox` | `sandbox`, `development` or `production`. Anything else, including a typo, silently means `sandbox`. |
| `PLAID_COUNTRY_CODES` | `US` | Comma-separated list passed to Plaid Link. |
| `PLAID_INTERVAL_IN_DAYS` | `30` | How far back a sync fetches transactions. |

## Scheduler

| Variable | Default | Notes |
| --- | --- | --- |
| `SCHEDULER` | `true` | Runs the in-process scheduler: recurring-expense generation, cleanup of expired sessions, tokens and cached rates, and scheduled backups. Every replica may run it; a database leader lock keeps each job to one replica. Setting it `false` does not stop the per-process janitor that clears expired background jobs. |
| `CLEAR_CACHE_CRON_RULE` | `0 3 * * *` | Accepted for compatibility with SplitPro configs. Not used: the rate-cache cleanup runs on every scheduler tick (once a minute), not on this rule. |
| `CACHE_RETENTION_INTERVAL` | `720h` | Cached exchange rates older than this are deleted by the cleanup. |

## Backup and restore

The behaviour behind these is described in [backup-restore.md](backup-restore.md).

| Variable | Default | Notes |
| --- | --- | --- |
| `BACKUP_DIR` | `./data/backups` | Where the CLI, the admin page and the scheduler write archives, and where an uploaded archive and the pre-restore safety dump are buffered. Not set by the container image; use `/data/backups` there. **Fatal** if it names the same directory as `AUTO_RESTORE_DIR`. |
| `BACKUP_CRON` | none | 5-field cron rule (UTC) for scheduled backups. Empty disables them. **Fatal** if the rule does not parse or `BACKUP_DIR` is empty. |
| `BACKUP_RETENTION_COUNT` | `7` | Keep this many newest archives in `BACKUP_DIR`; older ones are pruned as the last step of each scheduled backup, and only then. Without `BACKUP_CRON` (or with `SCHEDULER=false`) nothing is ever pruned. Negative keeps everything. `0` is **fatal**, since it would mean keep none. |
| `AUTO_RESTORE_DIR` | none | Checked at every start. If it holds exactly one archive and no `.gosplit-restored` marker, the archive is restored and the marker written. Empty disables the check. Must be writable (the marker and a safety dump go there) and must differ from `BACKUP_DIR`. **Fatal at boot** if two or more archives are present, if the directory cannot take the marker, or if validation or the restore fails; the boot keeps failing until the archive is removed. See [restoring at startup](backup-restore.md#restoring-at-startup). |
| `RESTORE_MAX_UPLOAD_MB` | `500` | Largest archive the admin page accepts for upload. |
| `RESTORE_MAX_ARCHIVE_MB` | `2048` | Largest decompressed archive a restore will process. |
| `RESTORE_MAX_ENTRIES` | `200000` | Most files (tables plus uploads) an archive may contain. |
| `RESTORE_MAX_TABLE_FILE_MB` | `512` | Largest single table file inside an archive. |
| `RESTORE_MAX_JSONL_LINE_MB` | `8` | Longest single row inside an archive. |

All five `RESTORE_MAX_*` values must be greater than zero; `0` is **fatal** rather than
meaning unlimited. They exist because an archive is a file that could come from anywhere.

## Accounts and features

| Variable | Default | Notes |
| --- | --- | --- |
| `ADMIN_EMAILS` | none | Comma-separated. A user whose address matches (case-insensitive) is promoted to admin when they register or sign in. Users are never demoted automatically. |
| `DISABLE_EMAIL_SIGNUP` | `false` | `true` closes self-service registration. The register page returns 403 and a magic-link request for an unknown address reports success without creating an account. Admins can still create users. |
| `ENABLE_SENDING_INVITES` | `true` | `false` stops users from adding a friend or group member by an email address that has no account yet. Picking an existing user still works. |
| `DEFAULT_HOMEPAGE` | `/balances` | Where a signed-in user lands. |
| `FEEDBACK_EMAIL` | none | Accepted for compatibility with SplitPro configs. Not used by any feature yet. |
| `DISCORD_WEBHOOK_URL` | none | Accepted for compatibility with SplitPro configs. Not used by any feature yet. |

## Example: local development

```sh
SESSION_SECRET=change-me-to-something-long-and-random
BASE_URL=http://localhost:8080
DATABASE_URL=file:./data/gosplit.db
# No SMTP: sign-in links are printed to the log.
ADMIN_EMAILS=you@example.com
```

## Example: production container

```sh
SESSION_SECRET=<openssl rand -base64 32>
BASE_URL=https://split.example.com
# DATABASE_URL, UPLOAD_DIR and ADDR come from the image; override only for PostgreSQL:
# DATABASE_URL=host=db port=5432 user=gosplit password=... dbname=gosplit sslmode=require
EMAIL_SERVER_HOST=smtp.example.com
EMAIL_SERVER_USER=gosplit
EMAIL_SERVER_PASSWORD=...
FROM_EMAIL=gosplit@example.com
ADMIN_EMAILS=you@example.com
DISABLE_EMAIL_SIGNUP=true
BACKUP_DIR=/data/backups
BACKUP_CRON=0 4 * * *
BACKUP_RETENTION_COUNT=14
```

## See also

- [deployment.md](deployment.md) -- where these settings go in Compose, Docker and behind a proxy
- [backup-restore.md](backup-restore.md) -- what the backup settings control
- [features.md](features.md) -- the features these settings switch on and off
