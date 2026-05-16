#!/usr/bin/env bash
# Idempotent end-to-end deploy of InterviewPrep to Google Cloud.
#
#   - Cloud SQL (Postgres 17 + pgvector) for the database
#   - Cloud Run service "interviewprep-api"  (Go backend, port 8080)
#   - Cloud Run service "interviewprep-fe"   (nginx + React SPA, port 8080)
#   - Cloud Run job     "interviewprep-backfill" (one-shot embeddings backfill)
#   - HTTPS Load Balancer with Google-managed SSL fronting both services
#   - Secret Manager for JWT_SECRET, GOOGLE_CLIENT_SECRET, DATABASE_URL
#   - Public GCS bucket for TTS audio output
#
# Every step uses describe-or-create so re-runs converge state. Safe to invoke
# any number of times; only `gcloud run deploy` and secret versions ever change
# on a no-op re-run.
#
# Usage:
#   PROJECT_ID=my-prj DOMAIN=example.com ADMIN_EMAILS=me@x.com \
#     GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=... \
#     ./deploy.sh
#
# Optional overrides: REGION, DB_INSTANCE, DB_NAME, DB_USER, API_SERVICE,
# FE_SERVICE, AUDIO_BUCKET, RUNTIME_SA, JWT_SECRET, DB_PASSWORD.

set -euo pipefail

# ─── Auto-load .env from repo root if present ──────────────────────────────
# Lets you keep config in .env and run `./deploy.sh` directly — no need for
# `set -a; source .env`. Existing exported env vars take precedence (they
# override what's in .env), so you can still do one-off overrides like
# `DOMAIN=foo.com ./deploy.sh`.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/.env" ]]; then
  echo "Loading config from ${SCRIPT_DIR}/.env"
  set -a
  # shellcheck disable=SC1091
  source "${SCRIPT_DIR}/.env"
  set +a
fi

# ─── Configuration ─────────────────────────────────────────────────────────
: "${PROJECT_ID:?set PROJECT_ID (e.g. PROJECT_ID=interviewprep-prod)}"
: "${REGION:=us-central1}"
: "${DOMAIN:?set DOMAIN (e.g. DOMAIN=10xinterview.com)}"
: "${ADMIN_EMAILS:?set ADMIN_EMAILS (comma-separated)}"
: "${GOOGLE_CLIENT_ID:?set GOOGLE_CLIENT_ID}"
: "${GOOGLE_CLIENT_SECRET:?set GOOGLE_CLIENT_SECRET}"

: "${DB_INSTANCE:=interviewprep-db}"
: "${DB_NAME:=interviewprep}"
: "${DB_USER:=interviewprep}"
: "${API_SERVICE:=interviewprep-api}"
: "${FE_SERVICE:=interviewprep-fe}"
: "${BACKFILL_JOB:=interviewprep-backfill}"
: "${AUDIO_BUCKET:=${PROJECT_ID}-audio}"
: "${RUNTIME_SA:=interviewprep-runtime}"
: "${LB_NAME:=interviewprep}"
: "${CERT_NAME:=${LB_NAME}-cert}"
: "${IP_NAME:=${LB_NAME}-ip}"

# Auto-generated on first run; re-runs reuse what's already in Secret Manager
# (we only write to SM if the secret name doesn't exist yet — see upsert_secret).
: "${JWT_SECRET:=$(openssl rand -base64 48)}"
: "${DB_PASSWORD:=$(openssl rand -base64 32 | tr -d '/+=' | head -c 32)}"

gcloud config set project "$PROJECT_ID" >/dev/null

log()  { printf '\n\033[1;34m▶ %s\033[0m\n' "$*"; }
done_() { printf '  \033[0;32m✓\033[0m %s\n' "$*"; }

# ─── 1. Enable APIs (idempotent: enable is a no-op if already enabled) ─────
log "Enabling required APIs"
gcloud services enable \
  run.googleapis.com \
  sqladmin.googleapis.com \
  secretmanager.googleapis.com \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  compute.googleapis.com \
  aiplatform.googleapis.com \
  speech.googleapis.com \
  texttospeech.googleapis.com \
  storage.googleapis.com \
  iam.googleapis.com
done_ "APIs enabled"

# ─── 2. Runtime service account + IAM bindings ─────────────────────────────
log "Service account: ${RUNTIME_SA}"
SA_EMAIL="${RUNTIME_SA}@${PROJECT_ID}.iam.gserviceaccount.com"
if ! gcloud iam service-accounts describe "$SA_EMAIL" >/dev/null 2>&1; then
  gcloud iam service-accounts create "$RUNTIME_SA" \
    --display-name="InterviewPrep Cloud Run runtime"
fi
for ROLE in \
  roles/cloudsql.client \
  roles/secretmanager.secretAccessor \
  roles/aiplatform.user \
  roles/speech.client \
  roles/storage.objectAdmin ; do
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="$ROLE" \
    --condition=None \
    --quiet >/dev/null
done
done_ "SA ${SA_EMAIL} has cloudsql.client, secretAccessor, aiplatform.user, speech.client, storage.objectAdmin"

# ─── 3. Cloud SQL (Postgres 17 + pgvector) ─────────────────────────────────
log "Cloud SQL instance: ${DB_INSTANCE}"
if ! gcloud sql instances describe "$DB_INSTANCE" >/dev/null 2>&1; then
  # ENTERPRISE edition supports shared-core tiers like db-f1-micro.
  # (Project default may be ENTERPRISE_PLUS, which only allows perf-optimized
  # tiers and would reject db-f1-micro.)
  #
  # pgvector is GA on Cloud SQL — the `vector` extension is available by
  # default. Migration 0007_search.sql runs `CREATE EXTENSION IF NOT EXISTS
  # vector;` on first backend boot. No --database-flags needed.
  echo "  Creating ${DB_INSTANCE} (~5 minutes)..."
  gcloud sql instances create "$DB_INSTANCE" \
    --project="$PROJECT_ID" \
    --region="$REGION" \
    --database-version=POSTGRES_17 \
    --edition=enterprise \
    --tier=db-f1-micro \
    --storage-size=10GB \
    --storage-auto-increase \
    --backup \
    --enable-point-in-time-recovery
  done_ "Created ${DB_INSTANCE}"
else
  done_ "Instance exists; skipping create"
fi

if ! gcloud sql users list --instance="$DB_INSTANCE" \
       --format='value(name)' | grep -qx "$DB_USER"; then
  gcloud sql users create "$DB_USER" \
    --instance="$DB_INSTANCE" --password="$DB_PASSWORD"
  done_ "Created DB user ${DB_USER}"
else
  done_ "DB user ${DB_USER} already exists (password unchanged)"
fi

if ! gcloud sql databases describe "$DB_NAME" \
       --instance="$DB_INSTANCE" >/dev/null 2>&1; then
  gcloud sql databases create "$DB_NAME" --instance="$DB_INSTANCE"
  done_ "Created database ${DB_NAME}"
else
  done_ "Database ${DB_NAME} already exists"
fi

INSTANCE_CONN=$(gcloud sql instances describe "$DB_INSTANCE" \
                  --format='value(connectionName)')
done_ "Connection name: ${INSTANCE_CONN}"

# Cloud Run reaches Cloud SQL via the unix socket mounted under /cloudsql.
# lib/pq accepts `host=` as a query param pointing at the socket directory.
DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@/${DB_NAME}?host=/cloudsql/${INSTANCE_CONN}&sslmode=disable"

# ─── 4. Secret Manager ─────────────────────────────────────────────────────
log "Secret Manager"
upsert_secret() {
  # Only writes a new version if the secret didn't exist (so DATABASE_URL
  # keeps the same DB_PASSWORD on re-runs, matching what's in Cloud SQL).
  # Empty values are skipped — gcloud rejects 0-byte payloads, and we'd
  # rather leave optional secrets (Gemini key, Razorpay creds) unmounted than
  # crash a first-run deploy where the user hasn't supplied them yet.
  local NAME=$1 VALUE=$2
  if [[ -z "$VALUE" ]]; then
    if gcloud secrets describe "$NAME" >/dev/null 2>&1; then
      done_ "Secret ${NAME} exists; leaving versions as-is"
    else
      done_ "Skipping ${NAME} (no value provided)"
    fi
    return
  fi
  if gcloud secrets describe "$NAME" >/dev/null 2>&1; then
    done_ "Secret ${NAME} already exists (not overwriting)"
  else
    printf %s "$VALUE" | gcloud secrets create "$NAME" \
      --data-file=- --replication-policy=automatic >/dev/null
    done_ "Created secret ${NAME}"
  fi
}
upsert_secret jwt-secret               "$JWT_SECRET"
upsert_secret google-oauth-client-secret "$GOOGLE_CLIENT_SECRET"
upsert_secret database-url             "$DATABASE_URL"
# Premium Gemini key + Razorpay credentials. First-run defaults are empty;
# `upsert_secret` won't overwrite, so once you push real values with
# `gcloud secrets versions add` they survive subsequent re-runs.
upsert_secret gemini-api-key           "${GEMINI_API_KEY:-}"
upsert_secret razorpay-key-id          "${RAZORPAY_KEY_ID:-}"
upsert_secret razorpay-key-secret      "${RAZORPAY_KEY_SECRET:-}"
upsert_secret razorpay-webhook-secret  "${RAZORPAY_WEBHOOK_SECRET:-}"

# ─── 5. Audio bucket (public read for TTS playback) ────────────────────────
log "GCS audio bucket: gs://${AUDIO_BUCKET}"
if ! gcloud storage buckets describe "gs://${AUDIO_BUCKET}" >/dev/null 2>&1; then
  gcloud storage buckets create "gs://${AUDIO_BUCKET}" \
    --location="$REGION" --uniform-bucket-level-access
  done_ "Created bucket"
else
  done_ "Bucket exists"
fi
gcloud storage buckets add-iam-policy-binding "gs://${AUDIO_BUCKET}" \
  --member=allUsers --role=roles/storage.objectViewer --quiet >/dev/null
done_ "allUsers granted objectViewer (public read)"

# ─── 6. Deploy backend (Cloud Run service) ─────────────────────────────────
log "Deploying ${API_SERVICE} (Cloud Run, Go)"

# Required secrets are always mounted. Optional secrets (Gemini key, Razorpay)
# are mounted only when they exist with a version — Cloud Run would fail to
# start if we referenced a missing :latest, so we probe first.
SECRET_MOUNTS="JWT_SECRET=jwt-secret:latest"
SECRET_MOUNTS+="|GOOGLE_CLIENT_SECRET=google-oauth-client-secret:latest"
SECRET_MOUNTS+="|DATABASE_URL=database-url:latest"
maybe_mount_secret() {
  local ENV_NAME=$1 SECRET_NAME=$2
  if gcloud secrets versions describe latest --secret="$SECRET_NAME" >/dev/null 2>&1; then
    SECRET_MOUNTS+="|${ENV_NAME}=${SECRET_NAME}:latest"
  fi
}
maybe_mount_secret GEMINI_API_KEY           gemini-api-key
maybe_mount_secret RAZORPAY_KEY_ID          razorpay-key-id
maybe_mount_secret RAZORPAY_KEY_SECRET      razorpay-key-secret
maybe_mount_secret RAZORPAY_WEBHOOK_SECRET  razorpay-webhook-secret

gcloud run deploy "$API_SERVICE" \
  --source="./backend" \
  --region="$REGION" \
  --platform=managed \
  --service-account="$SA_EMAIL" \
  --add-cloudsql-instances="$INSTANCE_CONN" \
  --allow-unauthenticated \
  --port=8080 \
  --memory=1Gi \
  --cpu=1 \
  --min-instances=0 \
  --max-instances=10 \
  --set-env-vars="^|^APP_ENV=production|GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID}|GOOGLE_REDIRECT_URL=https://${DOMAIN}/auth/google/callback|FRONTEND_URL=https://${DOMAIN}|CORS_ORIGIN=https://${DOMAIN}|POST_LOGIN_REDIRECT=/|COOKIE_SECURE=true|COOKIE_DOMAIN=${DOMAIN}|AGENT_ENABLED=true|GOOGLE_CLOUD_PROJECT=${PROJECT_ID}|GOOGLE_CLOUD_LOCATION=${GOOGLE_CLOUD_LOCATION:-${REGION}}|AGENT_MODEL=${AGENT_MODEL:-gemini-2.5-flash}|GEMINI_PREMIUM_MODEL=${GEMINI_PREMIUM_MODEL:-gemini-2.5-pro}|STT_LANGUAGE=${STT_LANGUAGE:-en-US}|AUDIO_BUCKET=${AUDIO_BUCKET}|AUDIO_BUCKET_LOCATION=${AUDIO_BUCKET_LOCATION:-${REGION}}|TTS_VOICE=${TTS_VOICE:-en-US-Neural2-D}|TTS_PROMPT_VOICE=${TTS_PROMPT_VOICE:-en-US-Neural2-F}|ADMIN_EMAILS=${ADMIN_EMAILS}|RAZORPAY_PLAN_MONTHLY_ID=${RAZORPAY_PLAN_MONTHLY_ID:-}|RAZORPAY_PLAN_BIANNUAL_ID=${RAZORPAY_PLAN_BIANNUAL_ID:-}" \
  --set-secrets="^|^${SECRET_MOUNTS}" \
  --quiet
done_ "${API_SERVICE} deployed"

# ─── 7. Deploy frontend (Cloud Run service via Cloud Build) ────────────────
log "Deploying ${FE_SERVICE} (Cloud Run, nginx)"
FE_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/cloud-run-source-deploy/${FE_SERVICE}:$(date +%Y%m%d-%H%M%S)"

# Cloud Run --source auto-creates this repo on first deploy of the API above,
# so by the time we hit the frontend it exists. If you skip step 6 for any
# reason, also create the repo with:
#   gcloud artifacts repositories create cloud-run-source-deploy \
#     --repository-format=docker --location=$REGION
gcloud builds submit ./frontend \
  --config=./frontend/cloudbuild.yaml \
  --substitutions=_IMAGE="$FE_IMAGE" \
  --quiet

gcloud run deploy "$FE_SERVICE" \
  --image="$FE_IMAGE" \
  --region="$REGION" \
  --platform=managed \
  --allow-unauthenticated \
  --port=8080 \
  --memory=256Mi \
  --cpu=1 \
  --min-instances=0 \
  --max-instances=5 \
  --quiet
done_ "${FE_SERVICE} deployed"

# ─── 8. HTTPS Load Balancer ────────────────────────────────────────────────
log "HTTPS Load Balancer: ${LB_NAME}"

# 8a. Serverless NEGs (one per Cloud Run service)
for pair in "${API_SERVICE}-neg:${API_SERVICE}" "${FE_SERVICE}-neg:${FE_SERVICE}"; do
  NEG="${pair%%:*}" ; SVC="${pair##*:}"
  if ! gcloud compute network-endpoint-groups describe "$NEG" \
        --region="$REGION" >/dev/null 2>&1; then
    gcloud compute network-endpoint-groups create "$NEG" \
      --region="$REGION" \
      --network-endpoint-type=serverless \
      --cloud-run-service="$SVC"
    done_ "Created NEG ${NEG} → ${SVC}"
  else
    done_ "NEG ${NEG} exists"
  fi
done

# 8b. Backend services (global, one per NEG)
# IMPORTANT: serverless NEGs reject --protocol=HTTPS (they handle the hop to
# Cloud Run internally over HTTPS via Google's network). Leave protocol unset
# so port_name defaults to "http".
ensure_backend_service() {
  local BS=$1 NEG=$2
  if ! gcloud compute backend-services describe "$BS" --global >/dev/null 2>&1; then
    gcloud compute backend-services create "$BS" \
      --global --load-balancing-scheme=EXTERNAL_MANAGED
  else
    # Heal services created by older versions of this script that had
    # --protocol=HTTPS (which set port_name=https, incompatible with
    # serverless NEGs).
    local CUR_PORT
    CUR_PORT=$(gcloud compute backend-services describe "$BS" --global \
                --format='value(portName)' 2>/dev/null || echo "")
    if [[ "$CUR_PORT" == "https" ]]; then
      gcloud compute backend-services update "$BS" --global \
        --port-name=http --quiet >/dev/null
      done_ "Patched port_name on ${BS} (https → http)"
    fi
  fi
  if ! gcloud compute backend-services describe "$BS" --global \
         --format='value(backends.group)' | grep -q "$NEG"; then
    gcloud compute backend-services add-backend "$BS" \
      --global --network-endpoint-group="$NEG" \
      --network-endpoint-group-region="$REGION"
  fi
}
ensure_backend_service "${API_SERVICE}-bs" "${API_SERVICE}-neg"
ensure_backend_service "${FE_SERVICE}-bs"  "${FE_SERVICE}-neg"
done_ "Backend services attached to NEGs"

# 8c. URL map with path-based routing.
# The path-matcher is removed + re-added every run so adding a new route
# (like /sitemap.xml) doesn't require manually editing the URL map. Brief
# window during the swap routes everything to the default (frontend), which
# is fine for a deploy — the LB is back to correct routing in <1s.
if ! gcloud compute url-maps describe "${LB_NAME}-urlmap" >/dev/null 2>&1; then
  gcloud compute url-maps create "${LB_NAME}-urlmap" \
    --default-service="${FE_SERVICE}-bs"
  done_ "Created URL map"
fi
gcloud compute url-maps remove-path-matcher "${LB_NAME}-urlmap" \
  --path-matcher-name=routes --quiet 2>/dev/null || true
gcloud compute url-maps add-path-matcher "${LB_NAME}-urlmap" \
  --path-matcher-name=routes \
  --default-service="${FE_SERVICE}-bs" \
  --path-rules="/api/*=${API_SERVICE}-bs,/auth/*=${API_SERVICE}-bs,/health=${API_SERVICE}-bs,/sitemap.xml=${API_SERVICE}-bs" \
  --new-hosts="*"
done_ "URL map routes refreshed: /api/*, /auth/*, /health, /sitemap.xml → backend; default → frontend"

# 8d. Google-managed SSL cert for $DOMAIN
if ! gcloud compute ssl-certificates describe "$CERT_NAME" --global >/dev/null 2>&1; then
  gcloud compute ssl-certificates create "$CERT_NAME" \
    --global --domains="$DOMAIN"
  done_ "Created managed SSL cert for ${DOMAIN}"
else
  done_ "SSL cert exists"
fi

# 8e. Target HTTPS proxy + ensure cert is attached (heals after a cert
# delete/recreate, where the proxy survives but loses its cert reference).
if ! gcloud compute target-https-proxies describe "${LB_NAME}-https" --global >/dev/null 2>&1; then
  gcloud compute target-https-proxies create "${LB_NAME}-https" \
    --url-map="${LB_NAME}-urlmap" \
    --ssl-certificates="$CERT_NAME"
  done_ "Created HTTPS target proxy"
else
  CUR_CERTS=$(gcloud compute target-https-proxies describe "${LB_NAME}-https" \
                --global --format='value(sslCertificates)' 2>/dev/null || echo "")
  if [[ "$CUR_CERTS" != *"${CERT_NAME}"* ]]; then
    gcloud compute target-https-proxies update "${LB_NAME}-https" \
      --global --ssl-certificates="$CERT_NAME" --quiet >/dev/null
    done_ "(Re-)attached cert ${CERT_NAME} to HTTPS proxy"
  else
    done_ "HTTPS target proxy exists with cert attached"
  fi
fi

# 8f. Global static IP
if ! gcloud compute addresses describe "$IP_NAME" --global >/dev/null 2>&1; then
  gcloud compute addresses create "$IP_NAME" --global --ip-version=IPV4
  done_ "Reserved global static IP"
fi
LB_IP=$(gcloud compute addresses describe "$IP_NAME" --global --format='value(address)')

# 8g. Global forwarding rule on 443
if ! gcloud compute forwarding-rules describe "${LB_NAME}-fr" --global >/dev/null 2>&1; then
  gcloud compute forwarding-rules create "${LB_NAME}-fr" \
    --global \
    --load-balancing-scheme=EXTERNAL_MANAGED \
    --address="$IP_NAME" \
    --target-https-proxy="${LB_NAME}-https" \
    --ports=443
  done_ "Created forwarding rule on 443"
else
  done_ "Forwarding rule (443) exists"
fi

# 8h. HTTP listener on port 80.
#   - Needed because Google-managed SSL certs validate via HTTP-01 on :80.
#     If port 80 is unreachable, certs hang in FAILED_NOT_VISIBLE.
#   - Always serves a 301 redirect to HTTPS (better SEO, prevents accidental
#     mixed-content). Google's cert validator bypasses URL-map redirects for
#     /.well-known/acme-challenge/* automatically, so the redirect is safe.
# Create/refresh the redirect URL map via `import` (YAML). The equivalent
# `gcloud compute url-maps create --default-url-redirect-*` flags exist only
# on newer SDK versions; `import` works everywhere and is idempotent
# (create-or-replace).
REDIRECT_YAML="$(mktemp)"
cat > "$REDIRECT_YAML" <<EOF
name: ${LB_NAME}-redirect-urlmap
defaultUrlRedirect:
  httpsRedirect: true
  redirectResponseCode: MOVED_PERMANENTLY_DEFAULT
  stripQuery: false
EOF
gcloud compute url-maps import "${LB_NAME}-redirect-urlmap" \
  --source="$REDIRECT_YAML" --global --quiet >/dev/null
rm -f "$REDIRECT_YAML"
done_ "HTTP→HTTPS redirect URL map ensured"

if ! gcloud compute target-http-proxies describe "${LB_NAME}-http" --global >/dev/null 2>&1; then
  gcloud compute target-http-proxies create "${LB_NAME}-http" \
    --url-map="${LB_NAME}-redirect-urlmap"
  done_ "Created HTTP target proxy (redirect to HTTPS)"
else
  # Heal if an earlier version of this script wired it to the main URL map.
  gcloud compute target-http-proxies update "${LB_NAME}-http" \
    --url-map="${LB_NAME}-redirect-urlmap" --quiet >/dev/null
  done_ "HTTP target proxy uses redirect URL map"
fi

if ! gcloud compute forwarding-rules describe "${LB_NAME}-fr-http" --global >/dev/null 2>&1; then
  gcloud compute forwarding-rules create "${LB_NAME}-fr-http" \
    --global \
    --load-balancing-scheme=EXTERNAL_MANAGED \
    --address="$IP_NAME" \
    --target-http-proxy="${LB_NAME}-http" \
    --ports=80
  done_ "Created forwarding rule on 80"
else
  done_ "Forwarding rule (80) exists"
fi

# ─── 9. Embeddings backfill (Cloud Run Job) ────────────────────────────────
log "Cloud Run Job: ${BACKFILL_JOB}"
# Reuse the API image (Dockerfile builds both /app/server and /app/backfill-embeddings).
API_IMAGE=$(gcloud run services describe "$API_SERVICE" --region="$REGION" \
              --format='value(spec.template.spec.containers[0].image)')

# Deploy is idempotent (creates or updates).
gcloud run jobs deploy "$BACKFILL_JOB" \
  --image="$API_IMAGE" \
  --command="/app/backfill-embeddings" \
  --region="$REGION" \
  --service-account="$SA_EMAIL" \
  --set-cloudsql-instances="$INSTANCE_CONN" \
  --set-secrets="DATABASE_URL=database-url:latest" \
  --set-env-vars="GOOGLE_CLOUD_PROJECT=${PROJECT_ID},GOOGLE_CLOUD_LOCATION=${REGION}" \
  --quiet
done_ "Job ready (run manually: gcloud run jobs execute ${BACKFILL_JOB} --region=${REGION} --wait)"

# ─── Final summary ─────────────────────────────────────────────────────────
API_URL=$(gcloud run services describe "$API_SERVICE" --region="$REGION" --format='value(status.url)')
FE_URL=$(gcloud run services describe "$FE_SERVICE"   --region="$REGION" --format='value(status.url)')
CERT_STATUS=$(gcloud compute ssl-certificates describe "$CERT_NAME" --global \
                --format='value(managed.status)' 2>/dev/null || echo UNKNOWN)

cat <<EOF

═══════════════════════════════════════════════════════════════════════════
  Deploy complete.
═══════════════════════════════════════════════════════════════════════════

  Cloud Run (direct URLs, for debugging only):
    Backend : ${API_URL}
    Frontend: ${FE_URL}

  Load Balancer:
    Static IP   : ${LB_IP}
    Domain      : ${DOMAIN}
    Cert status : ${CERT_STATUS}   (will go ACTIVE after DNS propagates)

───────────────────────────────────────────────────────────────────────────
  Next steps to bring the domain online
───────────────────────────────────────────────────────────────────────────

  1. At your DNS provider, create an A record:
        ${DOMAIN}.   A   ${LB_IP}

     (Optional: also add 'www' pointing at the same IP.)

  2. Wait for the managed cert to provision. Check with:
        gcloud compute ssl-certificates describe ${CERT_NAME} --global \\
          --format='value(managed.status,managed.domainStatus)'
     PROVISIONING → ACTIVE typically takes 5–60 min once DNS resolves.

  3. In the Google Cloud Console → APIs & Services → Credentials, edit the
     OAuth client and set:
        Authorized JavaScript origins : https://${DOMAIN}
        Authorized redirect URIs      : https://${DOMAIN}/auth/google/callback

  4. (Optional) Backfill embeddings on existing rows:
        gcloud run jobs execute ${BACKFILL_JOB} --region=${REGION} --wait

  5. Verify:
        curl -sf https://${DOMAIN}/health      # → 200 OK
        open  https://${DOMAIN}                # → SPA loads, Google login works

═══════════════════════════════════════════════════════════════════════════
EOF
