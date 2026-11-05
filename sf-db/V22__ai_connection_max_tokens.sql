-- A per-subscription answer-length ceiling (data.maxTokens on ai_connections).
--
-- The column is JSONB, so there is nothing to ALTER: a new subscription is
-- given the default by the API when it is saved, and this backfills the
-- subscriptions that already existed before the field did. Without it an older
-- connection would carry no ceiling and fall through to the deployment default
-- on every question - the same number, but worth writing down so the value a
-- member sees and can raise is really on their subscription.
--
-- 4096 mirrors models.DefaultMaxTokens; keep the two in step if it changes.
-- Only rows missing the key are touched, so re-running this changes nothing and
-- a member who already set their own ceiling keeps it.
UPDATE ai_connections
SET data = jsonb_set(data, '{maxTokens}', '4096'::jsonb, true)
WHERE NOT (data ? 'maxTokens');
