-- Track when each recurrence should next fire. The scheduler advances this on
-- generation so due expenses are produced exactly once.
ALTER TABLE expense_recurrences ADD COLUMN next_run_at TEXT;
CREATE INDEX idx_recurrences_next ON expense_recurrences(next_run_at);
