# AI instruct 44

## CWAI subscription

In `~/shadai`, `~/cwclock`, `~/uprodit` and `~/strong-fish` I want to add CWCloud as a new AI provider.

API documentation: https://www.cwcloud.tech/docs/tutorials/cwai#use-the-api

The payload to use:

```shell
curl -X 'POST' \
  'https://api.cwcloud.tech/v1/ai/prompt' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -H 'X-Auth-Token: XXXXXX' \
  -d '{
  "adapter": "gpt4o",
  "messages": [
    {
      "role": "system",
      "message": "You're a tech assistant"
    },
    {
      "role": "user",
      "message": "I need help"
    }
  ],
  "settings": {}
}'
```

See `~/cwc-chat-mobile` and `~/comwork-cloud-ui` for the answer payload.

So use a placeholder and default value for API url: https://api.cwcloud.tech/v1/ai/prompt

And ApiKey.

List of models in the decision table:
* mistral-medium
* claude-sonnet
* claude-haiku
* gemini-flash

The model has to be passed in the `adapter` field.
