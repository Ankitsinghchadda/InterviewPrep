-- Content schema: categories (roles + topics), questions, interviews, submissions.

CREATE TABLE IF NOT EXISTS categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('role','topic')),
    description TEXT NOT NULL DEFAULT '',
    icon        TEXT NOT NULL DEFAULT '',
    sort_order  INT  NOT NULL DEFAULT 100,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS categories_kind_idx ON categories (kind);

CREATE TABLE IF NOT EXISTS questions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             TEXT UNIQUE,                       -- set for seeded questions, NULL for user-created
    title            TEXT NOT NULL,
    body             TEXT NOT NULL DEFAULT '',
    answer           TEXT NOT NULL,
    difficulty       TEXT NOT NULL DEFAULT 'medium' CHECK (difficulty IN ('easy','medium','hard')),
    answer_audio_url TEXT NOT NULL DEFAULT '',
    owner_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    is_public        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS questions_owner_idx ON questions (owner_id);
CREATE INDEX IF NOT EXISTS questions_difficulty_idx ON questions (difficulty);

CREATE TABLE IF NOT EXISTS question_categories (
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (question_id, category_id)
);

CREATE INDEX IF NOT EXISTS qc_category_idx ON question_categories (category_id);

CREATE TABLE IF NOT EXISTS interviews (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_ids UUID[] NOT NULL DEFAULT '{}',
    status       TEXT NOT NULL DEFAULT 'in_progress'
                 CHECK (status IN ('in_progress','completed','abandoned')),
    score        NUMERIC(5,2),
    summary      TEXT NOT NULL DEFAULT '',
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS interviews_user_idx ON interviews (user_id);

CREATE TABLE IF NOT EXISTS interview_questions (
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    question_id  UUID NOT NULL REFERENCES questions(id) ON DELETE RESTRICT,
    position     INT  NOT NULL,
    PRIMARY KEY (interview_id, question_id)
);

CREATE INDEX IF NOT EXISTS iq_interview_idx ON interview_questions (interview_id, position);

CREATE TABLE IF NOT EXISTS answer_submissions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id   UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    interview_id  UUID REFERENCES interviews(id) ON DELETE CASCADE,
    audio_url     TEXT NOT NULL DEFAULT '',
    transcript    TEXT NOT NULL DEFAULT '',
    feedback      TEXT NOT NULL DEFAULT '',
    strengths     TEXT[] NOT NULL DEFAULT '{}',
    improvements  TEXT[] NOT NULL DEFAULT '{}',
    score         NUMERIC(5,2),
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','transcribing','reviewing','complete','failed')),
    error_message TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS submissions_user_idx ON answer_submissions (user_id);
CREATE INDEX IF NOT EXISTS submissions_interview_idx ON answer_submissions (interview_id);
CREATE INDEX IF NOT EXISTS submissions_question_idx ON answer_submissions (question_id);
