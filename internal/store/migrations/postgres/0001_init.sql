-- PostgreSQL schema for GoSplit. Mirrors the SQLite schema exactly in shape.
-- Timestamps are TEXT (ISO-8601 UTC) so Go has a single scanning path across
-- engines. Money is BIGINT (int64) minor units. UUIDs generated in Go.

CREATE TABLE users (
    id                 BIGSERIAL PRIMARY KEY,
    name               TEXT NOT NULL DEFAULT '',
    email              TEXT NOT NULL UNIQUE,
    email_verified     TEXT,
    password_hash      TEXT,
    image              TEXT,
    currency           TEXT NOT NULL DEFAULT 'USD',
    default_currency   TEXT NOT NULL DEFAULT 'USD',
    preferred_language TEXT NOT NULL DEFAULT 'en',
    role               TEXT NOT NULL DEFAULT 'USER',
    deactivated_at     TEXT,
    banking_id         TEXT,
    hidden_friend_ids  TEXT NOT NULL DEFAULT '[]',
    created_at         TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires    TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE verification_tokens (
    identifier TEXT NOT NULL,
    token      TEXT NOT NULL,
    purpose    TEXT NOT NULL DEFAULT 'magic',
    expires    TEXT NOT NULL,
    PRIMARY KEY (identifier, token)
);
CREATE INDEX idx_vtokens_token ON verification_tokens(token);

CREATE TABLE groups (
    id                 BIGSERIAL PRIMARY KEY,
    public_id          TEXT NOT NULL UNIQUE,
    name               TEXT NOT NULL,
    image              TEXT,
    created_by         BIGINT NOT NULL REFERENCES users(id),
    default_currency   TEXT NOT NULL DEFAULT 'USD',
    simplify_debts     BOOLEAN NOT NULL DEFAULT false,
    archived_at        TEXT,
    splitwise_group_id TEXT,
    created_at         TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
);

CREATE TABLE group_users (
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_group_users_user ON group_users(user_id);

CREATE TABLE friendships (
    owner_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
    PRIMARY KEY (owner_id, friend_id)
);
CREATE INDEX idx_friendships_friend ON friendships(friend_id);

CREATE TABLE group_default_splits (
    group_id   BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    split_type TEXT NOT NULL,
    shares     TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (group_id)
);

CREATE TABLE friend_default_splits (
    owner_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    split_type TEXT NOT NULL,
    shares     TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (owner_id, friend_id)
);

CREATE TABLE expenses (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    category       TEXT NOT NULL DEFAULT 'general',
    amount         BIGINT NOT NULL,
    split_type     TEXT NOT NULL,
    expense_date   TEXT NOT NULL,
    currency       TEXT NOT NULL,
    paid_by        BIGINT NOT NULL REFERENCES users(id),
    added_by       BIGINT NOT NULL REFERENCES users(id),
    updated_by     BIGINT REFERENCES users(id),
    group_id       BIGINT REFERENCES groups(id),
    file_key       TEXT,
    transaction_id TEXT,
    recurrence_id  BIGINT,
    conversion_to_id TEXT,
    moved_from_id  TEXT,
    deleted_at     TEXT,
    deleted_by     BIGINT REFERENCES users(id),
    created_at     TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
    updated_at     TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
);
CREATE INDEX idx_expenses_paidby ON expenses(paid_by);
CREATE INDEX idx_expenses_group ON expenses(group_id);
CREATE INDEX idx_expenses_deleted ON expenses(deleted_at);
CREATE INDEX idx_expenses_date ON expenses(expense_date);

CREATE TABLE expense_participants (
    expense_id TEXT NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    amount     BIGINT NOT NULL,
    PRIMARY KEY (expense_id, user_id)
);
CREATE INDEX idx_ep_user ON expense_participants(user_id);

CREATE TABLE expense_notes (
    id         BIGSERIAL PRIMARY KEY,
    expense_id TEXT NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    note       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
);
CREATE INDEX idx_notes_expense ON expense_notes(expense_id);

CREATE TABLE expense_recurrences (
    id                  BIGSERIAL PRIMARY KEY,
    cron_expression     TEXT NOT NULL,
    job_name            TEXT NOT NULL UNIQUE,
    template_expense_id TEXT NOT NULL REFERENCES expenses(id),
    notified            BOOLEAN NOT NULL DEFAULT false,
    created_by          BIGINT NOT NULL REFERENCES users(id),
    created_at          TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
);

CREATE TABLE scheduler_locks (
    id         TEXT PRIMARY KEY,
    holder     TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE TABLE cached_currency_rates (
    from_currency TEXT NOT NULL,
    to_currency   TEXT NOT NULL,
    rate_date     TEXT NOT NULL,
    rate          TEXT NOT NULL,
    PRIMARY KEY (from_currency, to_currency, rate_date)
);

CREATE TABLE cached_bank_data (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data       TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
    PRIMARY KEY (user_id)
);

CREATE TABLE push_notifications (
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint     TEXT NOT NULL,
    subscription TEXT NOT NULL,
    PRIMARY KEY (user_id, endpoint)
);

CREATE TABLE app_metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

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
