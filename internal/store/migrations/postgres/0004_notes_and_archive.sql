-- Per-expense free-text note, plus transaction archiving (collapse). Collapsed
-- originals are MOVED into archived_expenses (+ participants); a synthetic
-- "Historical Transactions" expense replaces them and carries a CSV note.

ALTER TABLE expenses ADD COLUMN note TEXT NOT NULL DEFAULT '';

-- Archive tables mirror the live shape but carry no FKs: the move must never be
-- blocked by self-referential conversion/move links, and archived rows are a
-- historical record decoupled from live integrity.
CREATE TABLE archived_expenses (
    id                 TEXT PRIMARY KEY,
    name               TEXT   NOT NULL,
    category           TEXT   NOT NULL,
    amount             BIGINT NOT NULL,
    split_type         TEXT   NOT NULL,
    expense_date       TEXT   NOT NULL,
    currency           TEXT   NOT NULL,
    paid_by            BIGINT NOT NULL,
    added_by           BIGINT NOT NULL,
    updated_by         BIGINT,
    group_id           BIGINT,
    file_key           TEXT,
    transaction_id     TEXT,
    recurrence_id      BIGINT,
    conversion_to_id   TEXT,
    moved_from_id      TEXT,
    deleted_at         TEXT,
    deleted_by         BIGINT,
    created_at         TEXT   NOT NULL,
    updated_at         TEXT   NOT NULL,
    note               TEXT   NOT NULL DEFAULT '',
    archive_expense_id TEXT   NOT NULL,
    archived_at        TEXT   NOT NULL
);
CREATE INDEX idx_archived_expenses_archive ON archived_expenses(archive_expense_id);

CREATE TABLE archived_expense_participants (
    expense_id TEXT   NOT NULL,
    user_id    BIGINT NOT NULL,
    amount     BIGINT NOT NULL,
    PRIMARY KEY (expense_id, user_id)
);
