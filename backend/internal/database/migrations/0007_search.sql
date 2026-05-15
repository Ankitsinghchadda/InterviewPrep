-- Hybrid search infrastructure: pgvector embeddings + Postgres full-text.
-- Requires the `pgvector/pgvector:pg17` image (or pgvector installed manually).

CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE questions
  ADD COLUMN IF NOT EXISTS embedding vector(768);

ALTER TABLE questions
  ADD COLUMN IF NOT EXISTS search_text tsvector
    GENERATED ALWAYS AS (
      setweight(to_tsvector('english', coalesce(title,  '')), 'A') ||
      setweight(to_tsvector('english', coalesce(body,   '')), 'B') ||
      setweight(to_tsvector('english', coalesce(answer, '')), 'C') ||
      setweight(to_tsvector('english', coalesce(intent, '')), 'D')
    ) STORED;

CREATE INDEX IF NOT EXISTS questions_search_text_idx
  ON questions USING GIN (search_text);

CREATE INDEX IF NOT EXISTS questions_embedding_idx
  ON questions USING hnsw (embedding vector_cosine_ops);
