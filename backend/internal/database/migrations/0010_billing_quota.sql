-- 0010_billing_quota.sql
-- Adds the free/paid tier system: plan fields on users, an append-only
-- usage_events log for rolling weekly quotas, and a payment_events audit
-- table (idempotency for Razorpay webhooks). Idempotent so re-runs are
-- safe under the auto-migrator in database.go.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS plan TEXT NOT NULL DEFAULT 'free',
    ADD COLUMN IF NOT EXISTS plan_period TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS plan_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS plan_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS razorpay_customer_id     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS razorpay_subscription_id TEXT NOT NULL DEFAULT '';

DO $$ BEGIN
    ALTER TABLE users ADD CONSTRAINT users_plan_check CHECK (plan IN ('free','pro'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    ALTER TABLE users ADD CONSTRAINT users_plan_period_check CHECK (plan_period IN ('','monthly','biannual'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_plan_expires
    ON users(plan_expires_at) WHERE plan = 'pro';

-- usage_events: one row per billable AI action. Used to compute rolling
-- 7-day quotas per (user, kind). Append-only; never updated.
CREATE TABLE IF NOT EXISTS usage_events (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_usage_user_kind_time
    ON usage_events(user_id, kind, occurred_at DESC);

-- payment_events: Razorpay webhook audit log. provider_event_id is the
-- Razorpay event id and is the idempotency key — duplicate webhooks
-- conflict on insert and are no-ops.
CREATE TABLE IF NOT EXISTS payment_events (
    id                BIGSERIAL PRIMARY KEY,
    user_id           UUID REFERENCES users(id) ON DELETE SET NULL,
    provider          TEXT NOT NULL DEFAULT 'razorpay',
    provider_event_id TEXT NOT NULL UNIQUE,
    event_type        TEXT NOT NULL,
    amount            BIGINT,
    currency          TEXT,
    payload           JSONB NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payment_events_user_created
    ON payment_events(user_id, created_at DESC);
