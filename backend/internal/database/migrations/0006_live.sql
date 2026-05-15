-- Adds the "live" interview mode: an agentic, time-bounded interview where
-- the next question is generated dynamically from the running Q&A transcript
-- + the user's profile/resume.
--
-- Schema changes:
-- 1. Allow mode='live' on interviews.
-- 2. Allow source='live' on questions (each generated turn is a question row).
-- 3. duration_seconds on interviews — the timer budget. 0 for topic/adaptive.

ALTER TABLE interviews DROP CONSTRAINT IF EXISTS interviews_mode_check;
ALTER TABLE interviews
    ADD CONSTRAINT interviews_mode_check CHECK (mode IN ('topic', 'adaptive', 'live'));

-- Widen to the full superset (including 'ai-generated' from 0009) so re-runs
-- of this migration on an upgraded DB don't reject rows added later. The
-- migrator re-applies every file on every boot and expects idempotence.
ALTER TABLE questions DROP CONSTRAINT IF EXISTS questions_source_check;
ALTER TABLE questions
    ADD CONSTRAINT questions_source_check
    CHECK (source IN ('curated', 'user', 'adaptive', 'live', 'ai-generated'));

ALTER TABLE interviews
    ADD COLUMN IF NOT EXISTS duration_seconds INT NOT NULL DEFAULT 0;
