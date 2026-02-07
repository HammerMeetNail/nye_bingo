DROP INDEX IF EXISTS idx_ai_logs_user_feature_date;

ALTER TABLE ai_generation_logs
  DROP COLUMN IF EXISTS feature;

DROP TABLE IF EXISTS ai_premium_usage_monthly;
