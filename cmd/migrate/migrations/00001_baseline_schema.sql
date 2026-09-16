-- Baseline: the production schema at the point goose was adopted, generated
-- from a live schema dump so it describes prod exactly as it stood.
--
-- Every statement is IF NOT EXISTS, so this file is safe against an already
-- populated database: on prod it no-ops and simply records the version, while
-- on a fresh database it builds the whole schema. That property is specific to
-- a baseline; later migrations should not copy it.

-- +goose Up

CREATE TABLE IF NOT EXISTS audio_files (
    id TEXT PRIMARY KEY,
    user_id TEXT,  -- Made nullable for anonymous uploads (GDPR compliance)
    original_filename TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    format TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'uploading',
    lufs_target REAL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    duration_seconds INTEGER,
    -- Retain foreign key for when user_id is provided
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS cookie_consents (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    essential BOOLEAN NOT NULL DEFAULT TRUE,
    analytics BOOLEAN NOT NULL DEFAULT FALSE,
    functional BOOLEAN NOT NULL DEFAULT FALSE,
    consent_version TEXT NOT NULL DEFAULT '1.0',
    user_agent TEXT,
    ip_address TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    advertising   BOOLEAN NOT NULL DEFAULT FALSE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS customer_portal_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    provider_session_id TEXT NOT NULL,
    return_url TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY,  -- Our internal UUID
    user_id TEXT NOT NULL,
    subscription_id TEXT,
    -- Provider Information
    provider TEXT NOT NULL,  -- 'stripe' or 'mollie'
    provider_invoice_id TEXT UNIQUE NOT NULL,  -- Stripe invoice ID or Mollie equivalent
    -- Invoice Details
    invoice_number TEXT,  -- Human-readable invoice number
    amount_due INTEGER NOT NULL,  -- Amount in cents
    amount_paid INTEGER DEFAULT 0,  -- Amount paid in cents
    currency TEXT NOT NULL DEFAULT 'usd',
    status TEXT NOT NULL,  -- 'draft', 'open', 'paid', 'void', 'uncollectible'
    -- Billing Period
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    -- URLs
    invoice_pdf_url TEXT,  -- URL to download PDF
    hosted_invoice_url TEXT,  -- URL for customer to view/pay
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    due_date TIMESTAMP,
    paid_at TIMESTAMP,
    -- Provider-specific data
    metadata TEXT,  -- JSON blob
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS payment_customers (
    id TEXT PRIMARY KEY,  -- Our internal UUID
    user_id TEXT NOT NULL UNIQUE,
    provider TEXT NOT NULL,  -- 'stripe' or 'mollie'
    provider_customer_id TEXT UNIQUE NOT NULL,  -- Stripe customer ID (cus_xxx) or Mollie customer ID
    email TEXT NOT NULL,
    name TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS payment_transactions (
    id TEXT PRIMARY KEY,  -- Our internal UUID
    user_id TEXT NOT NULL,
    subscription_id TEXT,
    -- Provider Information
    provider TEXT NOT NULL,  -- 'stripe' or 'mollie'
    provider_transaction_id TEXT UNIQUE NOT NULL,  -- Stripe payment intent ID or Mollie payment ID
    provider_customer_id TEXT NOT NULL,
    -- Transaction Details
    amount INTEGER NOT NULL,  -- Amount in cents
    currency TEXT NOT NULL DEFAULT 'usd',
    status TEXT NOT NULL,  -- 'pending', 'succeeded', 'failed', 'refunded'
    type TEXT NOT NULL,  -- 'subscription', 'one_time', 'refund'
    -- Payment Method (optional, for display)
    payment_method_type TEXT,  -- 'card', 'ideal', 'sepa_debit', etc.
    payment_method_last4 TEXT,  -- Last 4 digits of card
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    paid_at TIMESTAMP,
    -- Provider-specific data
    metadata TEXT,  -- JSON blob
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS processing_jobs (
    id TEXT PRIMARY KEY,
    audio_file_id TEXT NOT NULL,
    user_id TEXT,  -- Nullable for anonymous users
    status TEXT NOT NULL DEFAULT 'queued',
    error_message TEXT,
    output_s3_key TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    progress_percentage INTEGER NOT NULL DEFAULT 0,
    output_format TEXT,
    target_lufs REAL,
    FOREIGN KEY (audio_file_id) REFERENCES audio_files(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS processing_jobs_backup (
    id TEXT,
    audio_file_id TEXT,
    status TEXT,
    error_message TEXT,
    output_s3_key TEXT,
    started_at NUM,
    completed_at NUM,
    created_at NUM,
    updated_at NUM,
    user_id TEXT,
    priority INT
);

CREATE TABLE IF NOT EXISTS schema_migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS subscription_plans (
    id TEXT PRIMARY KEY,  -- 'free', 'premium', 'professional'
    name TEXT NOT NULL,
    description TEXT,
    -- Pricing
    price_monthly INTEGER NOT NULL,  -- Price in cents (e.g., 999 for $9.99)
    price_yearly INTEGER,  -- Annual price in cents (optional)
    currency TEXT NOT NULL DEFAULT 'eur',
    -- Features & Limits
    processing_time_monthly INTEGER NOT NULL,  -- Monthly processing time limit in seconds
    max_file_size_mb INTEGER NOT NULL,  -- Max file size in MB
    priority_processing BOOLEAN DEFAULT FALSE,
    batch_processing BOOLEAN DEFAULT FALSE,
    api_access BOOLEAN DEFAULT FALSE,
    supported_formats TEXT DEFAULT 'mp3',  -- Comma-separated list: 'mp3', 'mp3,wav', 'mp3,wav,flac'
    -- Provider Price IDs (for quick lookup)
    stripe_price_id_monthly TEXT,
    stripe_price_id_yearly TEXT,
    mollie_price_id_monthly TEXT,
    mollie_price_id_yearly TEXT,
    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,  -- Our internal UUID
    user_id TEXT NOT NULL,
    -- Provider Information (generic fields)
    provider TEXT NOT NULL,  -- 'stripe' or 'mollie'
    provider_subscription_id TEXT UNIQUE NOT NULL,  -- Stripe subscription ID or Mollie subscription ID
    provider_customer_id TEXT NOT NULL,  -- Stripe customer ID or Mollie customer ID
    -- Subscription Details
    plan_id TEXT NOT NULL,  -- 'free', 'premium', 'professional'
    status TEXT NOT NULL,  -- 'active', 'canceled', 'past_due', 'trialing', 'incomplete'
    -- Billing
    current_period_start TIMESTAMP NOT NULL,
    current_period_end TIMESTAMP NOT NULL,
    cancel_at_period_end BOOLEAN DEFAULT FALSE,
    canceled_at TIMESTAMP,
    trial_start TIMESTAMP,
    trial_end TIMESTAMP,
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Provider-specific data (JSON for flexibility)
    metadata TEXT,  -- JSON blob for provider-specific fields
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_upload_stats (
    user_id TEXT PRIMARY KEY,  -- user_id serves as the ID for this table
    total_uploads INTEGER DEFAULT 0,
    total_processing_time_seconds INTEGER DEFAULT 0,
    uploads_this_week INTEGER DEFAULT 0,
    last_upload_at TIMESTAMP,
    week_reset_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processing_time_this_month INTEGER DEFAULT 0,
    month_reset_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP,
    auth_provider TEXT NOT NULL DEFAULT 'email',
    auth_provider_id TEXT,
    subscription_tier INTEGER NOT NULL DEFAULT 1,
    subscription_expires_at TIMESTAMP,
    name TEXT,
    subscription_status TEXT DEFAULT 'active',
    marketing_consent BOOLEAN NOT NULL DEFAULT FALSE,
    marketing_consent_at TIMESTAMP,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    email_verified_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS webhook_events (
    id TEXT PRIMARY KEY,  -- Our internal UUID
    provider TEXT NOT NULL,  -- 'stripe' or 'mollie'
    event_type TEXT NOT NULL,  -- e.g., 'payment_intent.succeeded'
    provider_event_id TEXT UNIQUE NOT NULL,  -- Stripe event ID or Mollie equivalent
    -- Payload
    payload TEXT NOT NULL,  -- Full JSON payload from webhook
    -- Processing Status
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMP,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);


CREATE INDEX IF NOT EXISTS idx_audio_files_user_id ON audio_files(user_id);
CREATE INDEX IF NOT EXISTS idx_audio_files_status ON audio_files(status);
CREATE INDEX IF NOT EXISTS idx_audio_files_created_at ON audio_files(created_at);
CREATE INDEX IF NOT EXISTS idx_cookie_consents_user_id ON cookie_consents(user_id);
CREATE INDEX IF NOT EXISTS idx_cookie_consents_created_at ON cookie_consents(created_at);
CREATE INDEX IF NOT EXISTS idx_customer_portal_sessions_user_id ON customer_portal_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_invoices_user_id ON invoices(user_id);
CREATE INDEX IF NOT EXISTS idx_invoices_subscription_id ON invoices(subscription_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires ON password_reset_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_payment_customers_user_id ON payment_customers(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_customers_provider_customer_id ON payment_customers(provider_customer_id);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_user_id ON payment_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_subscription_id ON payment_transactions(subscription_id);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_provider_transaction_id ON payment_transactions(provider_transaction_id);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_status ON payment_transactions(status);
CREATE INDEX IF NOT EXISTS idx_processing_jobs_audio_file_id ON processing_jobs(audio_file_id);
CREATE INDEX IF NOT EXISTS idx_processing_jobs_status ON processing_jobs(status);
CREATE INDEX IF NOT EXISTS idx_processing_jobs_user_id ON processing_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_processing_jobs_output_format ON processing_jobs(output_format);
CREATE INDEX IF NOT EXISTS idx_schema_migrations_version ON schema_migrations(version);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_provider_subscription_id ON subscriptions(provider_subscription_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_auth_provider ON users(auth_provider, auth_provider_id);
CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
CREATE INDEX IF NOT EXISTS idx_webhook_events_provider_event_id ON webhook_events(provider_event_id);
CREATE INDEX IF NOT EXISTS idx_webhook_events_processed ON webhook_events(processed);
CREATE INDEX IF NOT EXISTS idx_webhook_events_event_type ON webhook_events(event_type);

-- +goose Down
SELECT 1;
