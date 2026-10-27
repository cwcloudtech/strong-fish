# {{.SenderName}} أرسل إليك رسالة على StrongFish

أرسل إليك {{.SenderName}} الرسالة التالية على [strong-fish.com]({{.AppURL}}):

> {{.Content}}
{{if .AudioURL}}
[رسالة صوتية]({{.AudioURL}})
{{end}}{{if .VideoURL}}
[رسالة فيديو]({{.VideoURL}})
{{end}}
للرد، انتقل إلى [رسائلك الخاصة]({{.MessagesURL}}).
