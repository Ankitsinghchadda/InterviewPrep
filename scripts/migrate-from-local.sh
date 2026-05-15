#!/usr/bin/env bash
# Dump the local docker-compose Postgres DB (which already has questions,
# categories, embeddings, etc.) and load it into Cloud SQL — skipping the
# Vertex AI re-embedding step entirely.
#
# Prerequisites:
#   - docker-compose stack is running locally (`docker compose up -d db`)
#   - deploy.sh has been run at least once, so the backend has booted and
#     applied migrations against Cloud SQL (the schema must already exist
#     in the target — this script only ships DATA).
#   - `gcloud` is authenticated and pointed at the right project.
#
# Usage:
#   PROJECT_ID=my-prj DB_INSTANCE=interviewprep-db ./scripts/migrate-from-local.sh
#
# Optional: LOCAL_DB_SERVICE (default: db), LOCAL_DB_USER/NAME (default: interviewprep),
# DB_NAME (target), IMPORT_BUCKET, INCLUDE_USERS (default: true).

set -euo pipefail

# Auto-load .env from repo root (one level up from scripts/) if present.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
if [[ -f "${REPO_ROOT}/.env" ]]; then
  echo "Loading config from ${REPO_ROOT}/.env"
  set -a
  # shellcheck disable=SC1091
  source "${REPO_ROOT}/.env"
  set +a
fi

: "${PROJECT_ID:?set PROJECT_ID}"
: "${DB_INSTANCE:=interviewprep-db}"
: "${DB_NAME:=interviewprep}"

: "${LOCAL_DB_SERVICE:=db}"
: "${LOCAL_DB_USER:=interviewprep}"
: "${LOCAL_DB_NAME:=interviewprep}"

: "${IMPORT_BUCKET:=${PROJECT_ID}-sql-imports}"
: "${INCLUDE_USERS:=true}"      # set to "false" to start with no users on prod
: "${TRUNCATE_FIRST:=false}"    # set to "true" to wipe destination tables before load
                                # (needed when the backend's seed migration has already run
                                # and you want your local snapshot to be authoritative)

DUMP_FILE="/tmp/interviewprep-data-$(date +%Y%m%d-%H%M%S).sql"
OBJECT_NAME="$(basename "$DUMP_FILE")"

log() { printf '\n\033[1;34m▶ %s\033[0m\n' "$*"; }

# ─── 1. Sanity: local DB is up ─────────────────────────────────────────────
log "Checking local Postgres container"
if ! docker compose ps --services --status=running 2>/dev/null | grep -qx "$LOCAL_DB_SERVICE"; then
  echo "ERROR: docker-compose service '$LOCAL_DB_SERVICE' is not running."
  echo "       Run: docker compose up -d $LOCAL_DB_SERVICE"
  exit 1
fi

# ─── 2. Dump data-only from local ──────────────────────────────────────────
log "Dumping local data → $DUMP_FILE"

# Always-excluded tables: refresh_tokens (JWT-secret-bound, useless after migration)
EXCLUDES=(--exclude-table-data=refresh_tokens)
if [[ "$INCLUDE_USERS" != "true" ]]; then
  EXCLUDES+=(--exclude-table-data=users
            --exclude-table-data=user_profiles
            --exclude-table-data=interviews
            --exclude-table-data=interview_questions
            --exclude-table-data=answer_submissions)
fi

# --data-only:        skip schema (already migrated on prod)
# --no-owner/-privs:  strip ownership so the import user can apply it
#
# We intentionally do NOT use --disable-triggers: Cloud SQL forbids any user
# (even table owners) from disabling system constraint triggers — only the
# inaccessible cloudsqladmin role can do that. pg_dump --data-only orders
# tables by FK dependency, so loading without disabling triggers works as
# long as the schema has no circular FKs (ours doesn't).
{
  # Optionally prepend a TRUNCATE so the dump fully replaces destination data.
  # `interviewprep` owns these tables (created via migrations), so it can
  # TRUNCATE them. CASCADE handles the FK references.
  if [[ "$TRUNCATE_FIRST" == "true" ]]; then
    echo "BEGIN;"
    TRUNCATE_TABLES="answer_submissions, interview_questions, interviews, user_profiles, question_categories, questions, categories"
    if [[ "$INCLUDE_USERS" == "true" ]]; then
      TRUNCATE_TABLES="${TRUNCATE_TABLES}, users"
    fi
    echo "TRUNCATE ${TRUNCATE_TABLES} CASCADE;"
    echo "COMMIT;"
  fi

  docker compose exec -T "$LOCAL_DB_SERVICE" \
    pg_dump \
      --data-only \
      --no-owner --no-privileges \
      "${EXCLUDES[@]}" \
      -U "$LOCAL_DB_USER" "$LOCAL_DB_NAME"
} > "$DUMP_FILE"

DUMP_SIZE=$(du -h "$DUMP_FILE" | cut -f1)
echo "  Dump size: $DUMP_SIZE"

# ─── 3. Upload to GCS (import bucket) ──────────────────────────────────────
log "Uploading to gs://${IMPORT_BUCKET}/${OBJECT_NAME}"
if ! gcloud storage buckets describe "gs://${IMPORT_BUCKET}" >/dev/null 2>&1; then
  gcloud storage buckets create "gs://${IMPORT_BUCKET}" \
    --location="$(gcloud sql instances describe "$DB_INSTANCE" --format='value(region)')" \
    --uniform-bucket-level-access
fi

# `gcloud sql import` runs as the Cloud SQL service account; grant it read.
SQL_SA=$(gcloud sql instances describe "$DB_INSTANCE" \
          --format='value(serviceAccountEmailAddress)')
gcloud storage buckets add-iam-policy-binding "gs://${IMPORT_BUCKET}" \
  --member="serviceAccount:${SQL_SA}" \
  --role=roles/storage.objectViewer \
  --quiet >/dev/null

gcloud storage cp "$DUMP_FILE" "gs://${IMPORT_BUCKET}/${OBJECT_NAME}"

# ─── 4. Import into Cloud SQL ──────────────────────────────────────────────
# Run import AS the application user (interviewprep), not the default
# cloudsqlsuperuser. The dump uses --disable-triggers, which requires the
# importing role to OWN the tables; interviewprep owns them because the
# backend ran the schema migrations as that user.
log "Importing into Cloud SQL: ${DB_INSTANCE}/${DB_NAME} (as user ${LOCAL_DB_USER})"
echo "  (this can take a few minutes; gcloud will block until done)"
gcloud sql import sql "$DB_INSTANCE" \
  "gs://${IMPORT_BUCKET}/${OBJECT_NAME}" \
  --database="$DB_NAME" \
  --user="$LOCAL_DB_USER" \
  --quiet

# ─── 5. Cleanup ────────────────────────────────────────────────────────────
log "Cleaning up"
gcloud storage rm "gs://${IMPORT_BUCKET}/${OBJECT_NAME}" --quiet
rm -f "$DUMP_FILE"

cat <<EOF

═══════════════════════════════════════════════════════════════════════════
  Local data imported into Cloud SQL.
═══════════════════════════════════════════════════════════════════════════

  Skip the embeddings backfill — your dump already contains the embedding
  column values.

  Verify:
    gcloud sql connect ${DB_INSTANCE} --user=${LOCAL_DB_USER} --database=${DB_NAME}
    SELECT count(*), count(embedding) FROM questions;
EOF
