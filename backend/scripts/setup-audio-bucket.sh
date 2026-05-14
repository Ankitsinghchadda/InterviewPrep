#!/usr/bin/env bash
# Sets up the GCS bucket used to serve TTS-generated reference-answer audio.
# Idempotent — safe to re-run. Reads GOOGLE_CLOUD_PROJECT and
# GOOGLE_CLOUD_LOCATION from backend/.env so it stays consistent with the rest
# of the backend config. Optionally honors AUDIO_BUCKET if already set.
#
# Usage:
#   cd backend && ./scripts/setup-audio-bucket.sh

set -euo pipefail

cd "$(dirname "$0")/.."

# Load .env if present.
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

PROJECT_ID="${GOOGLE_CLOUD_PROJECT:-}"
# GCS doesn't accept "global" (Vertex AI does). Prefer an explicit override via
# AUDIO_BUCKET_LOCATION; otherwise reuse GOOGLE_CLOUD_LOCATION unless it's the
# Vertex-only "global" pseudo-region.
REGION="${AUDIO_BUCKET_LOCATION:-${GOOGLE_CLOUD_LOCATION:-us-central1}}"
if [ "$REGION" = "global" ] || [ -z "$REGION" ]; then
  REGION="us-central1"
fi
BUCKET="${AUDIO_BUCKET:-interview-prep-audio-${PROJECT_ID}}"

if [ -z "$PROJECT_ID" ]; then
  echo "ERROR: GOOGLE_CLOUD_PROJECT is not set in backend/.env" >&2
  exit 1
fi

cat <<EOF
== Config ==
Project: $PROJECT_ID
Region:  $REGION
Bucket:  $BUCKET

EOF

# ----------------------------------------------------------------------------
# 1) Figure out which identity will write to the bucket from the backend.
#    The backend uses Application Default Credentials (same as Vertex AI).
#    Two common cases:
#      a) Local dev: ADC comes from `gcloud auth application-default login`
#         → identity is your user account.
#      b) Server/key-file: GOOGLE_APPLICATION_CREDENTIALS points at a service
#         account JSON → identity is that service account's client_email.
# ----------------------------------------------------------------------------

echo "== Currently authenticated gcloud accounts =="
gcloud auth list --format="table(account,status)" || true
echo

if [ -n "${GOOGLE_APPLICATION_CREDENTIALS:-}" ] && [ -f "${GOOGLE_APPLICATION_CREDENTIALS:-}" ]; then
  SA_EMAIL=$(grep -o '"client_email"[[:space:]]*:[[:space:]]*"[^"]*"' "$GOOGLE_APPLICATION_CREDENTIALS" \
    | head -n1 | sed -E 's/.*"client_email"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
  if [ -n "$SA_EMAIL" ]; then
    MEMBER="serviceAccount:${SA_EMAIL}"
    echo "Backend identity (from GOOGLE_APPLICATION_CREDENTIALS): $MEMBER"
  else
    echo "ERROR: could not parse client_email from $GOOGLE_APPLICATION_CREDENTIALS" >&2
    exit 1
  fi
else
  # Falls back to the gcloud user whose ADC is active locally.
  USER_EMAIL=$(gcloud config get-value account 2>/dev/null || true)
  if [ -z "$USER_EMAIL" ]; then
    echo "ERROR: no gcloud account configured and GOOGLE_APPLICATION_CREDENTIALS unset" >&2
    exit 1
  fi
  MEMBER="user:${USER_EMAIL}"
  echo "Backend identity (gcloud ADC user): $MEMBER"
fi

echo
echo "== Service accounts that already exist in $PROJECT_ID =="
gcloud iam service-accounts list --project="$PROJECT_ID" --format="table(email,displayName)" || true
echo
echo "If one of these is what your backend uses, re-run after exporting:"
echo "    export GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa-key.json"
echo "and the script will detect it. Otherwise it'll use your gcloud user."
echo

# ----------------------------------------------------------------------------
# 2) Enable required APIs (idempotent).
# ----------------------------------------------------------------------------
echo "== Enabling APIs (storage + text-to-speech) =="
gcloud services enable storage.googleapis.com texttospeech.googleapis.com \
  --project="$PROJECT_ID"

# ----------------------------------------------------------------------------
# 3) Create the bucket if it doesn't exist. Uniform bucket-level access keeps
#    IAM simple. We leave public-access-prevention at its default (inherited)
#    so the allUsers grant below can take effect — if your org policy blocks
#    that, the binding step will fail loudly and you'll need signed URLs.
# ----------------------------------------------------------------------------
if gcloud storage buckets describe "gs://$BUCKET" --project="$PROJECT_ID" >/dev/null 2>&1; then
  echo "Bucket gs://$BUCKET already exists — skipping create."
else
  echo "== Creating gs://$BUCKET in $REGION =="
  gcloud storage buckets create "gs://$BUCKET" \
    --project="$PROJECT_ID" \
    --location="$REGION" \
    --uniform-bucket-level-access
fi

# ----------------------------------------------------------------------------
# 4) Public read on every object — this is what makes <audio src=…> work
#    without auth. If your org policy blocks allUsers you'll get
#    "domainRestrictedSharing"; see the README workaround.
# ----------------------------------------------------------------------------
echo "== Granting allUsers objectViewer (public read) =="
gcloud storage buckets add-iam-policy-binding "gs://$BUCKET" \
  --member=allUsers \
  --role=roles/storage.objectViewer

# ----------------------------------------------------------------------------
# 5) Write access for whichever identity the backend authenticates as.
# ----------------------------------------------------------------------------
echo "== Granting $MEMBER objectAdmin (write) =="
gcloud storage buckets add-iam-policy-binding "gs://$BUCKET" \
  --member="$MEMBER" \
  --role=roles/storage.objectAdmin

cat <<EOF

✓ Done. Bucket: gs://$BUCKET

Add the following to backend/.env (if not already there):
    AUDIO_BUCKET=$BUCKET
    TTS_VOICE=en-US-Neural2-D

Public object URLs will be of the form:
    https://storage.googleapis.com/$BUCKET/<key>
EOF
