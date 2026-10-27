# {{.SenderName}} sent you a message on StrongFish

{{.SenderName}} sent you the following message on [strong-fish.com]({{.AppURL}}):

> {{.Content}}
{{if .AudioURL}}
[audio message]({{.AudioURL}})
{{end}}{{if .VideoURL}}
[video message]({{.VideoURL}})
{{end}}
To reply, go to your [private messages]({{.MessagesURL}}).
