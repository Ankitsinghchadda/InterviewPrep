-- Personal collections: named lists of questions a user assembles
-- ("Behavioral", "FAANG prep"). The default per-user "Saved" list (is_default
-- TRUE) is lazily created on first access and acts as a one-click bookmark.

CREATE TABLE IF NOT EXISTS collections (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '',
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS collections_user_idx ON collections (user_id);

-- One row per (collection, question). Membership is intrinsically unique on
-- the pair; collection_id ON DELETE CASCADE so deleting a collection
-- automatically drops its memberships.
CREATE TABLE IF NOT EXISTS collection_questions (
    collection_id UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    question_id   UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    added_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note          TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (collection_id, question_id)
);

CREATE INDEX IF NOT EXISTS collection_questions_question_idx
    ON collection_questions (question_id);
