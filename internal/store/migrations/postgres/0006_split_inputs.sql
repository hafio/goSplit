-- Saving a split now also records the RAW per-participant input the user typed,
-- not just the computed net amounts. Until this release only expenses.split_type
-- and expense_participants.amount survived, so the edit form had to invert the
-- amounts back into form values -- lossy for SHARE (it showed minor units where
-- the user typed weights) and impossible for ADJUSTMENT (silently re-saved as
-- EXACT).
--
-- The inputs live in their own table with a single detail column, deliberately:
-- they are reference data for restoring the form and are NEVER read to compute
-- balances, so they cannot corrupt the zero-sum participant rows. A missing row
-- means "no inputs recorded" -- a pre-0006 expense, or a system split
-- (SETTLEMENT/CURRENCY_CONVERSION/ARCHIVE) that never runs through the split
-- engine -- and the edit form falls back to the old reconstruction. Existing
-- rows are left alone: deriving values for them would freeze a guess into
-- storage, and for ADJUSTMENT it would simply be wrong.
--
-- The payload is versioned JSON ({"v":1,"method":...,"values":{userID:value}}),
-- so a future split method needs no migration.

CREATE TABLE expense_split_inputs (
    expense_id TEXT PRIMARY KEY REFERENCES expenses(id) ON DELETE CASCADE,
    inputs     TEXT NOT NULL
);

-- Optimistic concurrency for edits. Two members editing the same expense used to
-- produce a silent lost update: UpdateExpense rewrites the participant set, so
-- the second writer erased the first one's split with no warning. Every update
-- now asserts the version it read and bumps it, and a mismatch is reported to the
-- user instead of overwriting. updated_at could not serve as the token -- it is
-- millisecond precision, so two edits in the same millisecond compare equal.
--
-- Versions start at 1, not 0, so that the guard fails closed: a form that
-- carries no version token parses as 0 (see httpapp.formVersion), and with a
-- 0 default every never-edited row would have matched it.
ALTER TABLE expenses ADD COLUMN version BIGINT NOT NULL DEFAULT 1;
