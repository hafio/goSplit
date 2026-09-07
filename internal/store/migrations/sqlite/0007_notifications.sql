-- In-app notifications. Until this release the only way a participant learned
-- that someone had touched a shared expense was an email, and it fired on
-- create only: editing, deleting, settling and editing a settlement notified
-- nobody. Nothing was recorded either, so a bounced mail or a dismissed push
-- toast lost the event outright.
--
-- This table is the durable record and the default channel. Every notifiable
-- event writes one row per affected participant except the actor. Email
-- survives as a per-user opt-in (users.email_expense_notify below, default
-- OFF) and web push is driven from these same rows, so nothing can be
-- delivered that was not first recorded.
--
-- entity_type/entity_id carry the link to the subject and deliberately have NO
-- foreign key, for the same reason the archive tables in 0004 have none: a
-- notification is a historical record decoupled from live integrity. An FK
-- would either block deleting the expense or cascade the history away, and
-- both are wrong -- "A deleted the dinner expense" stays true after the
-- expense is gone. entity_id is TEXT because expense ids are UUIDs. actor_id
-- has no FK for the same reason; a deleted actor resolves through the same
-- name fallback every other feed uses.
--
-- title/amount/currency are a snapshot taken at write time, so the feed still
-- reads correctly after the expense is renamed, re-split or soft-deleted.
-- amount is the RECIPIENT's signed net share, not the expense total -- it is
-- what makes "you owe 12.50" renderable from the row alone.
--
-- No display text is stored. kind is an i18n key stem, so each viewer reads
-- the notification in their own preferred_language rather than in whatever
-- language the actor happened to be using.
--
-- read_at is NULL until the recipient clicks through to the entry or marks
-- everything read; the ISO-8601 .000Z layout is lexicographically comparable,
-- so the scheduler's retention purge is a plain string compare like sessions.

CREATE TABLE notifications (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id    INTEGER NOT NULL,
    kind        TEXT    NOT NULL,
    entity_type TEXT    NOT NULL,
    entity_id   TEXT    NOT NULL,
    title       TEXT    NOT NULL,
    amount      INTEGER NOT NULL DEFAULT 0,
    currency    TEXT    NOT NULL DEFAULT '',
    read_at     TEXT,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- The list query (newest first, one user) and the unread badge count, which
-- runs on every authenticated page render, each get their own covering index.
CREATE INDEX idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_user_read ON notifications(user_id, read_at);

-- Per-user opt-in for the expense email. Default 0: the notification row above
-- is always written and is the default channel, so email is purely additive
-- and nobody keeps receiving mail they never asked for.
ALTER TABLE users ADD COLUMN email_expense_notify INTEGER NOT NULL DEFAULT 0;
