# AI instruct 10

## Calendar bug

On calendar screen I have a 405 error with `GET https://api.strong-fish.com/v1/events`.
Fix it.

## UX/UI design sf-ui

Move the version just below the logo and the collapse button just before _feed_

## Private messages and block list

I want a private message features like it's implemented in most of social network.
You can MP only member which you have visibility.

A MP can be reported and members can be blocked (in this case no MP and no posts in the feed can be seen).

Blocklist can be updated to re-authorize members.

## IP adresses

I want to store every ip adresses in the user's payload with a connection counter.
Administrator can see the member's ip like in `~/uprodit` with counters (no ban unlike uprodit for now, it'll be manage by firewall).

## Observability

Like `~/cwclock` I want observability logs, traces and metrics (Prometheus route /v1/metrics and OTEL grpc configuration).
