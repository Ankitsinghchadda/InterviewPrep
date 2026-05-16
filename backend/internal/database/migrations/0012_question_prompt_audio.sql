-- 0012_question_prompt_audio.sql
-- Adds prompt_audio_url for the synthesized question prompt (the interviewer
-- voice reading the question aloud). answer_audio_url is the candidate-facing
-- reference answer; prompt_audio_url is the interviewer asking it.
-- Idempotent so re-runs are safe under the auto-migrator.

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS prompt_audio_url TEXT NOT NULL DEFAULT '';
