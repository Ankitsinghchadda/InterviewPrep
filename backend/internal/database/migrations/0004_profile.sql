-- Per-user profile: drives recommendations + adaptive interview generation.
-- Note: the column is `current_title` (not `current_role`) because CURRENT_ROLE
-- is a reserved SQL function name in PostgreSQL and triggers a parser error.
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id           UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    target_role       TEXT NOT NULL DEFAULT '',     -- category slug, e.g. 'backend'
    years_experience  INT  NOT NULL DEFAULT 0,
    seniority         TEXT NOT NULL DEFAULT ''
                      CHECK (seniority IN ('', 'junior', 'mid', 'senior', 'staff', 'principal')),
    current_title     TEXT NOT NULL DEFAULT '',     -- their current job title + company
    tech_stack        TEXT[] NOT NULL DEFAULT '{}',
    target_companies  TEXT[] NOT NULL DEFAULT '{}',
    goals             TEXT NOT NULL DEFAULT '',
    resume_text       TEXT NOT NULL DEFAULT '',
    resume_filename   TEXT NOT NULL DEFAULT '',
    onboarded_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
