CREATE TABLE ai_premium_usage_monthly (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    month_start DATE NOT NULL,
    enhancements_used INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, month_start)
);

ALTER TABLE ai_generation_logs
  ADD COLUMN feature VARCHAR(30) NOT NULL DEFAULT 'generate';

CREATE INDEX idx_ai_logs_user_feature_date ON ai_generation_logs(user_id, feature, created_at);
