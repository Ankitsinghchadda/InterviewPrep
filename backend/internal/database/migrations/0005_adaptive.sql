-- Distinguish curated/user/adaptive questions and topic-vs-adaptive interview mode.
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'curated';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.constraint_column_usage
        WHERE table_name = 'questions' AND constraint_name = 'questions_source_check'
    ) THEN
        ALTER TABLE questions
            ADD CONSTRAINT questions_source_check
            CHECK (source IN ('curated', 'user', 'adaptive'));
    END IF;
END$$;

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS intent TEXT NOT NULL DEFAULT '';

-- Backfill: existing seeded rows are 'curated', existing user-created are 'user'.
UPDATE questions SET source = 'user' WHERE owner_id IS NOT NULL AND source = 'curated';

ALTER TABLE interviews
    ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'topic';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.constraint_column_usage
        WHERE table_name = 'interviews' AND constraint_name = 'interviews_mode_check'
    ) THEN
        ALTER TABLE interviews
            ADD CONSTRAINT interviews_mode_check
            CHECK (mode IN ('topic', 'adaptive'));
    END IF;
END$$;

CREATE INDEX IF NOT EXISTS questions_source_idx ON questions(source);
