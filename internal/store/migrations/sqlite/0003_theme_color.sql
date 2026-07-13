-- Per-user accent theme (slug validated in the app against the theme allowlist).
-- Existing rows adopt the gentle burgundy default.
ALTER TABLE users ADD COLUMN theme_color TEXT NOT NULL DEFAULT 'burgundy';
