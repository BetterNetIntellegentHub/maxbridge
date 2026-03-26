CREATE TABLE IF NOT EXISTS telegram_groups (
    id BIGSERIAL PRIMARY KEY,
    telegram_chat_id BIGINT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
    readiness TEXT NOT NULL DEFAULT 'LIMITED' CHECK (readiness IN ('READY', 'LIMITED', 'BLOCKED')),
    readiness_reason TEXT NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS max_users (
    id BIGSERIAL PRIMARY KEY,
    max_user_id BIGINT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_blocked BOOLEAN NOT NULL DEFAULT FALSE,
    linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_delivery_status TEXT NOT NULL DEFAULT 'never',
    last_delivery_error TEXT NOT NULL DEFAULT '',
    last_delivery_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invites (
    id BIGSERIAL PRIMARY KEY,
    scope_type TEXT NOT NULL,
    scope_id TEXT NOT NULL,
    code_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    single_use BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_invites_scope ON invites(scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_invites_expires_at ON invites(expires_at);

CREATE TABLE IF NOT EXISTS routes (
    id BIGSERIAL PRIMARY KEY,
    telegram_chat_id BIGINT NOT NULL REFERENCES telegram_groups(telegram_chat_id) ON DELETE CASCADE,
    max_user_id BIGINT NOT NULL REFERENCES max_users(max_user_id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    filter_mode TEXT NOT NULL DEFAULT 'all' CHECK (filter_mode IN ('all', 'text_only', 'mentions_only')),
    ignore_bot_messages BOOLEAN NOT NULL DEFAULT TRUE,
    last_delivery_status TEXT NOT NULL DEFAULT 'never',
    last_delivery_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (telegram_chat_id, max_user_id)
);
CREATE INDEX IF NOT EXISTS idx_routes_lookup ON routes(telegram_chat_id, enabled);

CREATE TABLE IF NOT EXISTS dedupe_records (
    id BIGSERIAL PRIMARY KEY,
    dedupe_key TEXT NOT NULL UNIQUE,
    route_id BIGINT NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    telegram_chat_id BIGINT NOT NULL,
    telegram_message_id BIGINT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dedupe_expires ON dedupe_records(expires_at);

CREATE TABLE IF NOT EXISTS delivery_jobs (
    id BIGSERIAL PRIMARY KEY,
    route_id BIGINT NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    telegram_chat_id BIGINT NOT NULL,
    telegram_message_id BIGINT NOT NULL,
    max_user_id BIGINT NOT NULL REFERENCES max_users(max_user_id) ON DELETE CASCADE,
    payload_json JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'retry', 'completed', 'dead_letter')),
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 8,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    leased_until TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_delivery_jobs_sched ON delivery_jobs(status, available_at);
CREATE INDEX IF NOT EXISTS idx_delivery_jobs_lease ON delivery_jobs(status, leased_until);
CREATE INDEX IF NOT EXISTS idx_delivery_jobs_updated ON delivery_jobs(updated_at);

CREATE TABLE IF NOT EXISTS delivery_attempts (
    id BIGSERIAL,
    job_id BIGINT NOT NULL REFERENCES delivery_jobs(id) ON DELETE CASCADE,
    result TEXT NOT NULL CHECK (result IN ('success', 'temporary_error', 'permanent_error')),
    error_class TEXT NOT NULL DEFAULT '',
    error_detail TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS delivery_attempts_default PARTITION OF delivery_attempts DEFAULT;

DO $$
DECLARE
    month_start DATE := date_trunc('month', now())::date;
    next_month DATE := (date_trunc('month', now()) + interval '1 month')::date;
    month_after DATE := (date_trunc('month', now()) + interval '2 month')::date;
BEGIN
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS delivery_attempts_%s PARTITION OF delivery_attempts FOR VALUES FROM (%L) TO (%L)',
        to_char(month_start, 'YYYYMM'), month_start, next_month
    );
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS delivery_attempts_%s PARTITION OF delivery_attempts FOR VALUES FROM (%L) TO (%L)',
        to_char(next_month, 'YYYYMM'), next_month, month_after
    );
END $$;

CREATE INDEX IF NOT EXISTS idx_attempts_job_created ON delivery_attempts(job_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_attempts_created ON delivery_attempts(created_at);

CREATE TABLE IF NOT EXISTS app_events (
    id BIGSERIAL PRIMARY KEY,
    level TEXT NOT NULL,
    source TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_app_events_created ON app_events(created_at DESC);
