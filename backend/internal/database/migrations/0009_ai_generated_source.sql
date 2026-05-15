-- Allow source='ai-generated' on questions. These are produced by the
-- QuestionGenerator agent when a user requests AI-authored questions for a
-- skill that has few or no curated entries. They are stored as public catalog
-- rows (owner_id NULL) so every user benefits.

ALTER TABLE questions DROP CONSTRAINT IF EXISTS questions_source_check;
ALTER TABLE questions
    ADD CONSTRAINT questions_source_check
    CHECK (source IN ('curated', 'user', 'adaptive', 'live', 'ai-generated'));
