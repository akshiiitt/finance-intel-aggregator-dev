CREATE TABLE IF NOT EXISTS ai_quotas (
    quota_date    DATE PRIMARY KEY,
    gemini_calls  INTEGER NOT NULL DEFAULT 0,
    groq8b_calls  INTEGER NOT NULL DEFAULT 0,
    groq70b_calls INTEGER NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
