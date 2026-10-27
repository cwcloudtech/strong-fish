# AI instruct 34

## Chat's email notifications

When a chat message is sent, also send it by email with:

Title: `{Username} send you a message on StrongFish`

Body:

```markdown
{Username} send you the following message on [strong-fish.com](https://strong-fish.com):

> message content

To reply to this message, connect go to your [private messages](https://www.strong-fish.com/dashboard/messages).
```

I want a markdown template of the email I can update later and it has to be translated on French and Arabe depending on the language of the recipient.

If the message contains video or vocal, send also the link in the body formated like this:

```
[audio message](sf api link to the media)
[video message](sf api link to the media)
```
