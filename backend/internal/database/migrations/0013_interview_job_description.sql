-- 0013_interview_job_description.sql
-- Adds an optional job description on each interview. Used by the live
-- interviewer agent to tailor questions to a specific role the candidate is
-- targeting, on top of their resume/profile. Empty string means "not set" —
-- the prompt builder skips the section in that case.
-- Idempotent so re-runs are safe under the auto-migrator.

ALTER TABLE interviews
    ADD COLUMN IF NOT EXISTS job_description TEXT NOT NULL DEFAULT '';
