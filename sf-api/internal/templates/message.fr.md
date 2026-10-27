# {{.SenderName}} vous a envoyé un message sur StrongFish

{{.SenderName}} vous a envoyé le message suivant sur [strong-fish.com]({{.AppURL}}) :

> {{.Content}}
{{if .AudioURL}}
[message vocal]({{.AudioURL}})
{{end}}{{if .VideoURL}}
[message vidéo]({{.VideoURL}})
{{end}}
Pour répondre, rendez-vous dans vos [messages privés]({{.MessagesURL}}).
