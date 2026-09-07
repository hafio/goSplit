# Backup and restore

How to take a complete copy of a GoSplit instance and how to put one back. For operators.
Settings are listed in [configuration.md](configuration.md#backup-and-restore).

## What an archive is

An archive is one file with the extension `.gsbak`, named
`gosplit-backup-<UTC timestamp>.gsbak`, for example
`gosplit-backup-20260906T041500Z.gsbak`. It holds every database table, including
sessions, tokens and cached bank data, plus the whole upload tree. It does not hold the
balances view, which is derived from expenses and recomputed after a restore.

An archive is portable between engines. One taken from SQLite restores into PostgreSQL
and the other way round.

A restore is a **wipe and replace**. Every table is emptied and refilled with the archive's
rows, ids included, and the upload directory is swapped for the archive's copy. There is
no merge.

### Archives are secrets

An archive contains every password hash, every live session token and any bank data. It
is always encrypted (AES-256-GCM) with a key derived from `SESSION_SECRET`, and written
with mode `0600`.

There is no separate backup key and no override. **If you rotate or lose
`SESSION_SECRET`, every archive made under the old value becomes permanently
unrestorable.** Restore anything you still need before you rotate. Restoring with the
wrong secret fails early with a message that says so, rather than with a cryptic
decryption error.

The archive's header is plain text, so `gosplit inspect` can show when and where an
archive was made without the secret or a database.

## Four ways to make a backup

### From the command line

```sh
gosplit backup                 # writes to BACKUP_DIR
gosplit backup -o /some/dir    # writes somewhere else
```

Progress and the manifest summary (row counts per table, upload count) go to stderr. The
command needs the same `SESSION_SECRET` and `DATABASE_URL` as the server, so run it in the
same environment. In the container, which has no shell, exec the binary directly:

```sh
docker exec <container> /app/gosplit backup -o /data/backups
```

### Seeing where things are

The image has no shell, so there is no `ls` to check what is on the volume. `gosplit paths`
reports it instead -- every directory the app writes to, whether it exists and is
writable, its owner, the archives in each, and whether a startup restore marker is
present:

```sh
docker exec <container> /app/gosplit paths
```

It reads nothing from the database and changes nothing, so it is safe to run at any time.
It is the fastest way to answer "where did my backup go" and "why can the app not write
there".

If you point `-o` (or `BACKUP_DIR`) at a **bind-mounted** host directory, it has to be
writable by UID 65532, the user the container runs as -- a root-owned mount fails with
`permission denied`. `chown -R 65532:65532 <host path>` fixes it; see
[deployment](deployment.md#bind-mounts-must-be-owned-by-65532). Named volumes need
nothing.

`gosplit backup` refuses to run if the database has migrations this binary has not
applied yet. The message tells you to start the server once, which applies them, and try
again. This stops a pre-upgrade backup from silently migrating your data.

### From the admin page

`/admin/backup` (admins only) lists the archives in `BACKUP_DIR`, generates a new one and
lets you download any of them. Generation runs in the background; reload the page to see
the new archive in the list. A job still running after two hours is abandoned, and a
finished job's status is discarded fifteen minutes after it completes. The page warns
when you are on plain HTTP, since the download is sensitive.

### On a schedule

Set `BACKUP_CRON` to a 5-field cron rule, for example `0 4 * * *` for 04:00 UTC daily
(rules are always evaluated in UTC). After each run the newest `BACKUP_RETENTION_COUNT`
archives in `BACKUP_DIR` are kept and older ones deleted; pre-restore safety dumps there
are pruned to the same count. This is the only pruning there is: without a schedule,
archives from the CLI and the admin page simply accumulate. A database lock keeps the job
on one replica when several run the scheduler.

Copy archives off the machine. A backup on the same volume as the database is not a
disaster-recovery plan.

### On startup (auto-restore)

`AUTO_RESTORE_DIR` is the unattended path, described under
[restoring at startup](#restoring-at-startup).

## Inspecting an archive

```sh
gosplit inspect gosplit-backup-20260906T041500Z.gsbak
```

Prints the format version, creation time, source engine, the GoSplit version that made it,
the encryption parameters, the key fingerprint and the list of migrations it was taken
under. Needs neither `SESSION_SECRET` nor a database, so it is the safe first thing to
run on any archive.

## Restoring

### Check first

```sh
gosplit restore --file ARCHIVE
```

Without `--force` this validates the archive against the current database, prints its
manifest, and changes nothing in the database. It confirms three things:

- the archive was sealed with this `SESSION_SECRET`;
- the archive's migration set matches the database's exactly (see below);
- the archive is structurally sound and within the `RESTORE_MAX_*` limits, and no upload
  inside it exceeds `UPLOAD_MAX_FILE_SIZE_MB` (a backup archives every file whatever its
  size, so lowering that setting can make an older archive fail here).

Two filesystem side effects do happen on a check: the upload tree is staged into a sibling
directory of `UPLOAD_DIR` (and removed again), and any leftover from an interrupted earlier
restore is repaired first. Both are harmless, but a check is not a read-only operation on
disk.

### Replace everything

```sh
gosplit restore --file ARCHIVE --force
```

What happens, in order:

1. A **safety dump** of the current data is written to `BACKUP_DIR` as
   `gosplit-pre-restore-<timestamp>.gsbak`. If that fails, the restore stops before
   touching anything. This dump is your only way back, and it is subject to the same
   retention pruning as scheduled backups, so copy it elsewhere if you may need it later.
   `--no-pre-dump` skips this step. Do not use it unless you have a fresh backup in hand.
2. All tables are emptied and refilled in one transaction. Row counts are verified against
   the manifest before commit; any mismatch rolls the whole thing back. On PostgreSQL the
   id sequences are advanced past the restored ids.
3. The upload directory is swapped for the staged copy.

The database step is atomic. The upload swap cannot join the transaction, so it runs after
the commit as two renames. A crash between those two renames leaves restored rows with old
uploads; GoSplit detects that state at the next start (or the next CLI restore), moves the
old uploads back into place, and refuses to boot with a message telling you to re-run the
restore. A crash after the commit but before the first rename is not detected: the staged
copy is discarded and the instance boots with restored rows and the previous upload tree.
If a restore was interrupted at all, re-run it.

### While a restore runs

Treat any restore as a maintenance window; the site is effectively unavailable until it
finishes. When a restore is started from the admin page, every request that would change
data, anywhere on the site, gets `503` with a `Retry-After` header. Reads are not
refused, but they do not get through either:

- On **SQLite**, the restore holds the database's single connection for its duration, so
  every other request waits on it and fails at the handler's ten-second deadline. A CLI
  restore prints a reminder to this effect.
- On **PostgreSQL**, the restore takes an `ACCESS EXCLUSIVE` lock on every table, which
  blocks reads as well as writes from any process. The admin-page `503` gate applies here
  too.

### Migration sets must match exactly

An archive records the list of migrations the source database had applied. A restore is
refused unless the target has applied exactly the same list, no more and no fewer. The
comparison is by migration name, not by when it ran, so restoring into a freshly
created database works.

In practice: restore with the same GoSplit version that made the archive. If you are
also upgrading, restore first, then upgrade. The reason for the strictness is that some
migrations rewrite data; if one were allowed to run after a restore it would overwrite
restored values.

### Restoring from the admin page

`/admin/backup` accepts an upload (up to `RESTORE_MAX_UPLOAD_MB`), validates it, shows the
manifest summary, and asks you to type `RESTORE ALL DATA` to proceed. The phrase is fixed
and not translated. A wrong phrase leaves the staged archive in place so you can try
again; an abandoned upload is discarded after 15 minutes.

The restore replaces the users and sessions tables, so your own session is gone the
moment it commits. The progress page and its status poll are therefore gated by an
unguessable job token rather than by a session. When the job finishes, a request to that
URL signs you back in if your user exists in the restored data, and otherwise sends you to
the login page. The token stays valid until the job is swept fifteen minutes after it
finishes, so treat the URL as a credential: do not share it or let it into logs.

Uploads are buffered inside `BACKUP_DIR`, not the system temp directory, because the
container's root filesystem is read-only. Make sure the volume has room for two more
archives during an upload-and-restore (the upload and the safety dump both land in
`BACKUP_DIR`) plus an uncompressed copy of the archive's upload tree next to `UPLOAD_DIR`.

## Restoring at startup

Set `AUTO_RESTORE_DIR` to a directory the server can write. At every start, after
migrations and before the listener opens, GoSplit looks in it:

- If the marker file `.gosplit-restored` exists, nothing happens.
- If exactly one `.gsbak` archive is present (safety dumps named `gosplit-pre-restore-*`
  are ignored), it is validated and restored, and the marker is written. A safety dump of
  whatever was in the database goes into the same directory first.
- If no archive is present, the boot continues normally.

The intended use is disaster recovery and moving an instance: provision a fresh volume,
drop one archive into the watched directory, start the container. Remove the archive
afterwards; the marker prevents a repeat, but a stray archive is a loaded gun.

Rules to know:

- `AUTO_RESTORE_DIR` must differ from `BACKUP_DIR`, or the server would restore its own
  most recent scheduled backup. The configuration refuses to start otherwise.
- Two or more archives in the directory abort the boot. GoSplit will not guess which one.
- **Any restore failure aborts the boot**, a migration mismatch included, and the
  container will keep failing on every restart until you remove the archive or unset
  the variable. The log line names that one-step fix. This is deliberate: an instance
  that silently started without the data you asked for would be worse.
- Two failures are only warnings: a directory that does not exist, and losing the leader
  lock to another replica that is doing the restore. Both let the boot continue.
- The directory is probed for writability before anything is restored, because a marker
  that cannot be written would mean a repeat restore on every start.

For multi-replica PostgreSQL deployments, either point `AUTO_RESTORE_DIR` at one replica
only or use the CLI. The leader lock protects the database, but a losing replica simply
boots without restoring.

## Housekeeping

- Half-written archives are named `<archive>.partial` and renamed only when complete, so a
  scanner never sees a truncated file. A running server deletes partials older than 24
  hours from `BACKUP_DIR` only; one left in a `-o` directory or in `AUTO_RESTORE_DIR` is
  yours to remove.
- `BACKUP_RETENTION_COUNT` applies separately to `gosplit-backup-*` and
  `gosplit-pre-restore-*` files in `BACKUP_DIR`, and only runs after a scheduled backup.
  Safety dumps written by auto-restore into `AUTO_RESTORE_DIR` are never pruned.
- `/profile/export` is unrelated: it is a per-user JSON takeout, not a backup, and it
  cannot be restored.
- Exit codes are `0` for success and `1` for any failure. `backup` and `restore` print
  progress and the manifest to stderr and `inspect` prints to stdout, but a failure (a
  pending-migration refusal, a wrong secret) is reported as a JSON log record on stdout,
  so capture both streams when scripting. `-h` on a subcommand prints usage and exits `1`.

## Not supported, by decision

- A backup key separate from `SESSION_SECRET`, or a re-key command.
- Incremental or differential archives. Retention count is the size control.
- Restoring the balances view. It is derived and must stay that way.

## See also

- [configuration.md](configuration.md#backup-and-restore) -- the settings
- [deployment.md](deployment.md) -- volumes, upgrades and where to put the backup directory
- [architecture.md](architecture.md#backup-archives) -- the archive format in more detail
