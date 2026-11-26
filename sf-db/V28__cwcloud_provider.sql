-- CWCloud (CWAI) as a provider a member can subscribe to.
--
-- It is not a vendor like the six already in the catalogue: it is comwork's
-- own prompt API in front of them. One CWCloud key reaches whichever adapters
-- that account has been granted - a Mistral, a Claude, a Gemini - which is
-- worth a row here for the coach who already pays comwork for CWCloud and
-- would rather not open a second account with a model vendor as well.
--
-- Two consequences the catalogue has to carry:
--
--   * the model is named in CWAI's `adapter` field, so what a connection calls
--     its "model" here is an adapter name on the CWCloud side. The suggestions
--     below are the adapters CWCloud publishes today; the field stays free
--     text, because that list is theirs to change and a form that refuses a
--     new one is a form nobody can use.
--   * the format is new ('cwcloud', the fourth the API speaks). V18 left the
--     format column free text with no CHECK over it, so there is no constraint
--     to widen here - the format is validated where it is read, by the client's
--     switch, which treats anything it does not know as OpenAI-compatible.
--
-- The default address is the whole endpoint rather than a root, because that is
-- the line CWCloud's own documentation gives people to paste. The client
-- accepts the host or the /v1 root too and appends what is missing.
INSERT INTO ai_providers (name, format, base_url, default_model, position, data) VALUES
    ('cwcloud', 'cwcloud', 'https://api.cwcloud.tech/v1/ai/prompt', 'claude-sonnet', 7,
        jsonb_build_object(
        'label', 'CWCloud (CWAI)',
        'keyHint', 'the CWCloud API token (X-Auth-Token)',
        'models', jsonb_build_array('mistral-medium', 'claude-sonnet', 'claude-haiku',
            'gemini-flash')
    ))
ON CONFLICT (name) DO UPDATE SET
    format = EXCLUDED.format,
    base_url = EXCLUDED.base_url,
    default_model = EXCLUDED.default_model,
    data = EXCLUDED.data,
    updated_at = now();
