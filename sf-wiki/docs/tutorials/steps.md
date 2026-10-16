---
id: steps
title: Steps from your watch
sidebar_position: 6
---

StrongFish can show the steps your watch already counted, next to the rest of
your training. It reads them; it never writes anything back, and it keeps no
copy - the numbers come from your account every time you look.

## What it connects to

**Google Health**, which is where Fitbit data lives now. Fitbit's own developer
API was switched off on 30 October 2026, and a Fitbit, a Pixel Watch or a phone
that records steps all report into the same Google Health profile.

So the account you connect is your **Google account**, and what it reads is
whatever your devices have put in Google Health.

## Before you connect

Your watch has to be linked to Google Health already. For a Fitbit that means
opening the **Google Health app** once and linking your Fitbit account to your
Google account.

This catches people out: the Google consent screen will happily say yes for an
account that has no Google Health profile behind it, and nothing would ever
arrive. StrongFish checks at the moment you connect and tells you if that is
the case, rather than leaving you with an empty chart.

## Connecting

1. Open **Settings** on the web app, or your **profile** on the phone.
2. Under _Connected devices_, choose **Connect** next to Google Health.
3. Google asks you to sign in and to allow StrongFish to read your activity.
4. You land back on StrongFish, connected.

Only one permission is asked for: reading activity and fitness. Not your sleep,
not your weight, and nothing at all to write.

On the phone, consent opens in your browser rather than inside the app - so you
can see whose login page you are typing into. Come back to the app afterwards
and the connection is there.

## What you see

Steps per day, in whichever window you pick:

| View | What it covers |
| --- | --- |
| Last 24h | Today's count |
| Last 48h | Today and yesterday |
| Last week | Seven days, with the average per 24h |
| Last month | Thirty days, with the average per 24h |
| Last 90 days | Ninety days, with the average per 24h |

These are whole days, not rolling windows: "last 24h" is today's total, the
same number the watch itself shows. Ninety days is the longest view there is,
because it is the longest range Google returns in one request.

Each day is coloured on the usual walking scale - under 6,000 steps, under
10,000, under 15,000, and above it - and so is the per-24h figure. Totals over
several days are left uncoloured: a month of walking adds up to a number that
would sit in the top band whatever you actually did.

## Disconnecting

**Disconnect** in the same place removes StrongFish's access from its side, and
the numbers stop being read.

It does not revoke anything in your Google account - that is yours to do, from
[your Google account's permissions page](https://myaccount.google.com/permissions),
and StrongFish deliberately does not reach in and do it for you.

## For whoever runs the server

It reuses the OAuth client that already signs people in with Google -
`SF_OIDC_GOOGLE_CLIENT_ID` and `SF_OIDC_GOOGLE_CLIENT_SECRET`. There is no
second pair of credentials: it is the same Google Cloud project, and one OAuth
client asks for different permissions per request - an identity when somebody
signs in, activity when they connect a watch.

Four things have to be true of that project. The old single "OAuth consent
screen" page no longer exists - it is now three pages under **Google Auth
Platform**, which is why these are given as direct links.

1. The **Google Health API** is enabled:
   [API Library](https://console.cloud.google.com/apis/library/health.googleapis.com).
2. `<your API URL>/v1/devices/google-health/callback` is one of the OAuth
   client's authorized redirect URIs:
   [Credentials](https://console.cloud.google.com/apis/credentials). Signing in
   and connecting a watch come back to different routes, so adding this one is
   a step of its own.
3. The Google Health scope is listed on
   [Data Access](https://console.cloud.google.com/auth/scopes) - *Add or remove
   scopes*, search for Google Health API.
4. Whoever is testing is on the test-user list:
   [Audience](https://console.cloud.google.com/auth/audience), where
   *Publishing status* reads **Testing** and *User type* **External**. Add each
   address under **Test users**.

With Google sign-in not configured at all, the section still appears, marked as
not enabled here - a member can see the feature exists rather than wondering.

Two things to plan for while the app is in Testing:

- It is capped at **100 test users**, each added by hand.
- A test user's authorization **expires seven days after they consent**, so the
  connection drops weekly until the app is published. StrongFish says so
  plainly - "the connection has expired, connect the account again" - rather
  than showing an empty chart.

Both limits go away once the app is published and verified. In a Google
Workspace organization, an admin marking the app **Trusted** lifts them for
that organization's own users.
