-- The providers the AI assistant knows how to talk to.
--
-- Its own migration rather than an edit to V17: that one is committed, and a
-- migration that has been applied anywhere is frozen - Flyway checksums it,
-- and a deployment that has already run it would refuse to start.
--
-- A table rather than a switch in the code, because none of it is code: a
-- provider is an address, a protocol, a default model and the names of the
-- models it serves. All four move - a vendor renames a model, an operator
-- points the whole deployment at a gateway - and moving them should be an
-- UPDATE, not a release.

CREATE TABLE ai_providers (
    name TEXT PRIMARY KEY,
    format TEXT NOT NULL,
    base_url TEXT NOT NULL,
    default_model TEXT NOT NULL,
    -- data: {label, models: [...], keyHint} - what to call it on screen, the
    -- models to suggest (best first), and what its key looks like, so a form
    -- can show an example rather than an empty box.
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- The order a picker lists them in.
    position INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The providers ~/cwclock speaks to, with the addresses it defaults to.
INSERT INTO ai_providers (name, format, base_url, default_model, position, data) VALUES
    ('openai', 'openai', 'https://api.openai.com/v1', 'gpt-4o-mini', 1, jsonb_build_object(
        'label', 'OpenAI',
        'keyHint', 'sk-...',
        'models', jsonb_build_array('gpt-4.1', 'gpt-4.1-mini', 'gpt-4o', 'gpt-4o-mini', 'o4-mini', 'o3')
    )),
    ('mistral', 'openai', 'https://api.mistral.ai/v1', 'mistral-small-latest', 2, jsonb_build_object(
        'label', 'Mistral',
        'models', jsonb_build_array('mistral-large-latest', 'mistral-medium-latest',
            'mistral-small-latest', 'magistral-medium-latest', 'open-mistral-nemo', 'codestral-latest')
    )),
    ('anthropic', 'anthropic', 'https://api.anthropic.com/v1', 'claude-sonnet-5-5', 3, jsonb_build_object(
        'label', 'Anthropic (Claude)',
        'keyHint', 'sk-ant-...',
        'models', jsonb_build_array('claude-opus-5-5', 'claude-sonnet-5-5', 'claude-haiku-5-5',
            'claude-sonnet-4-5', 'claude-haiku-4-5')
    )),
    ('gemini', 'gemini', 'https://generativelanguage.googleapis.com/v1beta', 'gemini-2.5-flash', 4,
        jsonb_build_object(
        'label', 'Google Gemini',
        'keyHint', 'AIza...',
        'models', jsonb_build_array('gemini-2.5-pro', 'gemini-2.5-flash', 'gemini-2.0-flash')
    )),
    -- One key for every vendor at once, which is why its model names carry
    -- the vendor in front.
    ('openrouter', 'openai', 'https://openrouter.ai/api/v1', 'meta-llama/llama-3.3-70b-instruct', 5,
        jsonb_build_object(
        'label', 'OpenRouter',
        'keyHint', 'sk-or-...',
        'models', jsonb_build_array('meta-llama/llama-3.3-70b-instruct', 'anthropic/claude-sonnet-4.5',
            'openai/gpt-4o-mini', 'google/gemini-2.5-flash', 'deepseek/deepseek-chat',
            'qwen/qwen-2.5-72b-instruct')
    )),
    ('deepseek', 'openai', 'https://api.deepseek.com/v1', 'deepseek-chat', 6, jsonb_build_object(
        'label', 'DeepSeek',
        'models', jsonb_build_array('deepseek-chat', 'deepseek-reasoner')
    ));
