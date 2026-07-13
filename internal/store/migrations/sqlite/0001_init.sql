-- SQLite schema for GoSplit. Timestamps are stored as ISO-8601 TEXT (UTC) on
-- both engines so Go has a single scanning path. Money is INTEGER (int64) minor
-- units. hidden_friend_ids is a JSON text array. UUIDs (expenses) are generated
-- in Go, not by the DB.

CREATE TABLE users (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT    NOT NULL DEFAULT '',
    email              TEXT    NOT NULL UNIQUE,
    email_verified     TEXT,
    password_hash      TEXT,
    image              TEXT,
    currency           TEXT    NOT NULL DEFAULT 'USD',
    default_currency   TEXT    NOT NULL DEFAULT 'USD',
    preferred_language TEXT    NOT NULL DEFAULT 'en',
    role               TEXT    NOT NULL DEFAULT 'USER',
    deactivated_at     TEXT,
    banking_id         TEXT,
    hidden_friend_ids  TEXT    NOT NULL DEFAULT '[]',
    created_at         TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires    TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

-- Magic-link / email-verification / password-reset tokens.
CREATE TABLE verification_tokens (
    identifier TEXT NOT NULL,
    token      TEXT NOT NULL,
    purpose    TEXT NOT NULL DEFAULT 'magic',
    expires    TEXT NOT NULL,
    PRIMARY KEY (identifier, token)
);
CREATE INDEX idx_vtokens_token ON verification_tokens(token);

CREATE TABLE groups (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    public_id          TEXT    NOT NULL UNIQUE,
    name               TEXT    NOT NULL,
    image              TEXT,
    created_by         INTEGER NOT NULL REFERENCES users(id),
    default_currency   TEXT    NOT NULL DEFAULT 'USD',
    simplify_debts     INTEGER NOT NULL DEFAULT 0,
    archived_at        TEXT,
    splitwise_group_id TEXT,
    created_at         TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE group_users (
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_group_users_user ON group_users(user_id);

-- Default splits: split_type + shares JSON. group_id XOR (friend of) owner.
-- Explicit friend links (a friendship exists before any shared expense).
CREATE TABLE friendships (
    owner_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (owner_id, friend_id)
);
CREATE INDEX idx_friendships_friend ON friendships(friend_id);

CREATE TABLE group_default_splits (
    group_id   INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    split_type TEXT    NOT NULL,
    shares     TEXT    NOT NULL DEFAULT '{}',
    PRIMARY KEY (group_id)
);

CREATE TABLE friend_default_splits (
    owner_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    split_type TEXT    NOT NULL,
    shares     TEXT    NOT NULL DEFAULT '{}',
    PRIMARY KEY (owner_id, friend_id)
);

CREATE TABLE expenses (
    id             TEXT PRIMARY KEY,           -- UUID generated in Go
    name           TEXT    NOT NULL,
    category       TEXT    NOT NULL DEFAULT 'general',
    amount         INTEGER NOT NULL,           -- int64 minor units
    split_type     TEXT    NOT NULL,
    expense_date   TEXT    NOT NULL,           -- ISO date/time
    currency       TEXT    NOT NULL,
    paid_by        INTEGER NOT NULL REFERENCES users(id),
    added_by       INTEGER NOT NULL REFERENCES users(id),
    updated_by     INTEGER REFERENCES users(id),
    group_id       INTEGER REFERENCES groups(id),
    file_key       TEXT,
    transaction_id TEXT,
    recurrence_id  INTEGER,
    conversion_to_id TEXT,                     -- links a CURRENCY_CONVERSION pair
    moved_from_id  TEXT,                       -- set on the replacement created by a move
    deleted_at     TEXT,
    deleted_by     INTEGER REFERENCES users(id),
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_expenses_paidby ON expenses(paid_by);
CREATE INDEX idx_expenses_group ON expenses(group_id);
CREATE INDEX idx_expenses_deleted ON expenses(deleted_at);
CREATE INDEX idx_expenses_date ON expenses(expense_date);

CREATE TABLE expense_participants (
    expense_id TEXT    NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    amount     INTEGER NOT NULL,              -- signed int64; rows sum to zero
    PRIMARY KEY (expense_id, user_id)
);
CREATE INDEX idx_ep_user ON expense_participants(user_id);

CREATE TABLE expense_notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    expense_id TEXT    NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    note       TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_notes_expense ON expense_notes(expense_id);

CREATE TABLE expense_recurrences (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    cron_expression     TEXT    NOT NULL,
    job_name            TEXT    NOT NULL UNIQUE,
    template_expense_id TEXT    NOT NULL REFERENCES expenses(id),
    notified            INTEGER NOT NULL DEFAULT 0,
    created_by          INTEGER NOT NULL REFERENCES users(id),
    created_at          TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE scheduler_locks (
    id         TEXT PRIMARY KEY,
    holder     TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE TABLE cached_currency_rates (
    from_currency TEXT    NOT NULL,
    to_currency   TEXT    NOT NULL,
    rate_date     TEXT    NOT NULL,
    rate          TEXT    NOT NULL,           -- decimal string, parsed in Go
    PRIMARY KEY (from_currency, to_currency, rate_date)
);

CREATE TABLE cached_bank_data (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data       TEXT    NOT NULL,
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (user_id)
);

CREATE TABLE push_notifications (
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint     TEXT    NOT NULL,
    subscription TEXT    NOT NULL,
    PRIMARY KEY (user_id, endpoint)
);

CREATE TABLE app_metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Derived balances. amount > 0 in row (user_id, friend_id) means friend_id
-- owes user_id. Per currency; group_id NULL = direct friend balance.
CREATE VIEW balance_view AS
WITH pair AS (
    SELECT
        CASE WHEN ep.user_id < e.paid_by THEN ep.user_id ELSE e.paid_by END AS a,
        CASE WHEN ep.user_id < e.paid_by THEN e.paid_by ELSE ep.user_id END AS b,
        e.group_id AS group_id,
        e.currency AS currency,
        ep.amount * (CASE WHEN ep.user_id < e.paid_by THEN 1 ELSE -1 END) AS signed
    FROM expense_participants ep
    JOIN expenses e ON e.id = ep.expense_id
    WHERE ep.user_id <> e.paid_by
      AND e.deleted_at IS NULL
)
SELECT a AS user_id, b AS friend_id, group_id, currency, SUM(signed) AS amount
FROM pair GROUP BY a, b, group_id, currency
UNION ALL
SELECT b AS user_id, a AS friend_id, group_id, currency, -SUM(signed) AS amount
FROM pair GROUP BY a, b, group_id, currency;
