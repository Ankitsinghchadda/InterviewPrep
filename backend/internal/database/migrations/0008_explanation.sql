-- Learner-facing explanation columns. Populated lazily by the explanation
-- agent on first request and persisted so subsequent loads are instant.

ALTER TABLE questions
  ADD COLUMN IF NOT EXISTS explanation_summary  TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS explanation_markdown TEXT NOT NULL DEFAULT '';
