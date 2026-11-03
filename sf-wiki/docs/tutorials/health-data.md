---
id: health-data
title: Health data
sidebar_position: 6
---

StrongFish can show what your watch and your scale already measured - steps, heart rate, weight, body fat, muscle mass and the rest - next to your training. It reads them; it never writes anything back, and it keeps no copy: the numbers come from your accounts every time you look.

## What it connects to

| Account | What it reads |
| --- | --- |
| **Google Health** (Fitbit, Pixel Watch, phone) | Steps and heart rate |
| **Withings** (scales, watches) | Body composition, plus steps and heart rate |
| **Stance Fitness** (bar-speed tracker) | Bar speed and measured RPE on your last squat, bench and deadlift |

**Google Health** is where Fitbit data lives now. Fitbit's own developer API was switched off on 30 October 2026, and a Fitbit, a Pixel Watch or a phone that records steps all report into the same Google Health profile. So the account you connect is your **Google account**, and what it reads is whatever your devices have put in Google Health.

**Withings** is connected with your Withings account, the one your scale syncs to.

**Stance Fitness** is a bar-speed tracker - a small accelerometer that clips to the bar. You pair it with your Stance account, and the tracking itself happens in the Stance app on your phone, which is where the bar is actually measured. StrongFish reads back your last set of each lift, and on the phone it can open the Stance app for you to record the next one.

You can connect any of them, in any combination.

### When both are connected

Steps and heart rate come from **one account at a time**, so the same day is never counted twice:

- Google Health is used whenever it is connected.
- Withings is used for steps and heart rate only when no Google Health account is connected.

Body composition always comes from Withings, whichever account the steps come from. The Withings row says so when its steps are not the ones shown.

## Before you connect

**For Google Health**, your watch has to be linked to Google Health already. For a Fitbit that means opening the **Google Health app** once and linking your Fitbit account to your Google account.

This catches people out: the Google consent screen will happily say yes for an account that has no Google Health profile behind it, and nothing would ever arrive. StrongFish checks at the moment you connect and tells you if that is the case, rather than leaving you with an empty chart.

**For Withings**, your scale only needs to be set up in the Withings app as usual.

**For Stance**, install the Stance app on the same phone and sign in to it once. That is the account you pair, and the Stance app is what captures the bar - so it has to be the phone you lift with.

## Connecting

1. Open **Health data** on the web app or the phone.
2. Choose **Connect** next to Google Health or Withings.
3. The provider asks you to sign in and to allow StrongFish to read your data.
4. You land back on StrongFish, connected.

Only read permissions are asked for:

- **Google Health**: activity and fitness, and heart rate. Not your sleep, not your weight, and nothing at all to write.
- **Withings**: your measures and your daily activity.

**Stance** is the exception to the sign-in dance: there is no provider consent screen. You pair it by typing your Stance email and password straight into StrongFish, which hands them to Stance once in exchange for a token. The password itself is never stored.

On the phone, consent opens in your browser rather than inside the app - so you can see whose login page you are typing into. Come back to the app afterwards and the connection is there.

## What you see

### Steps

Steps per day, in whichever window you pick:

| View | What it covers |
| --- | --- |
| Last 24h | Today's count |
| Last 48h | Today and yesterday |
| Last week | Seven days, with the average per 24h |
| Last 14 days | Fourteen days, with the average per 24h |
| Last month | Thirty days, with the average per 24h |
| Last 90 days | Ninety days, with the average per 24h |

These are whole days, not rolling windows: "last 24h" is today's total, the same number the watch itself shows. Ninety days is the longest view there is, because it is the longest range Google returns in one request.

Each day is coloured on the usual walking scale - under 6,000 steps, under 10,000, under 15,000, and above it - and so is the per-24h figure. Totals over several days are left uncoloured: a month of walking adds up to a number that would sit in the top band whatever you actually did.

### Heart rate

Over the same days, a bar per day as tall as its highest heart rate, coloured by the hardest zone reached: blue for light, yellow for moderate, orange for vigorous, red for peak. The zones are your provider's own, worked out from your own maximum heart rate.

Google Health only goes back **14 days** for heart rate. On the wider views the steps are still shown in full, along with your last heart rate reading, but not a heart rate chart that would cover less than its title says.

### Body composition

With Withings connected, a **Body composition** panel shows the latest value of every measure your scale takes, each with the date it was measured:

| Measure | Unit |
| --- | --- |
| Weight | kg |
| Body fat | % |
| Fat mass | kg |
| Fat-free mass | kg |
| Muscle mass | kg |
| Body water | kg |
| Bone mass | kg |
| Visceral fat | index |
| Basal metabolic rate | kcal |
| BMI | kg/m² (from your latest weight and the height in your Withings profile) |

A measure your scale does not take simply does not appear. Each one has its own date because a scale does not take every measure every time - fat, for instance, is only read barefoot.

### Bar speed

With Stance paired, a **Bar speed** panel shows your last set of each of the three lifts - squat, bench and deadlift. Each set is a row of bars, one per rep, each as tall as that rep's bar speed, above the load and reps and the two RPEs: the one you gave, and the one Stance measured from the bar.

Only the most recent set of each lift is shown - the point is the shape of your last effort, not a log. Record a new set and it takes the old one's place, and a lift you have not tracked simply does not appear.

On the phone, a **Record a set** button opens the Stance app so you can track the next set; come back afterwards and it is there. The web has no such button - the phone is where the bar is measured - but a set you record on the phone shows up on the web all the same.

### How the measured RPE is worked out

The measured RPE comes from the bar's speed: the slower a rep runs compared with how fast that load usually moves for you, the closer the set is to failure. Stance works it out from your set, and StrongFish reads and charts the result - it does none of the estimation itself. For the method behind it, written as plain sports science rather than anything tied to one tracker, see **[Velocity-based effort](./velocity-based-rpe.md)**.

## Exporting

**Export this view** saves what is on screen as a PNG or a PDF: steps and heart rate drawn as histograms, coloured like the screen, followed by a table of your latest body composition when Withings is connected, and the same bar-speed charts for your last squat, bench and deadlift when Stance is paired.

## Disconnecting

**Disconnect** in the same place removes StrongFish's access from its side, and the numbers stop being read.

It does not revoke anything in your provider account - that is yours to do, from [your Google account's permissions page](https://myaccount.google.com/permissions) or the connected apps of your Withings account, and StrongFish deliberately does not reach in and do it for you. For Stance there is no such page to visit - disconnecting drops the token StrongFish holds, and that is the whole of it.

## For whoever runs the server

An integration whose credentials are not set still appears, marked as not enabled here - a member can see the feature exists rather than wondering.

### Google Health

It reuses the OAuth client that already signs people in with Google - `SF_OIDC_GOOGLE_CLIENT_ID` and `SF_OIDC_GOOGLE_CLIENT_SECRET`. There is no second pair of credentials: it is the same Google Cloud project, and one OAuth client asks for different permissions per request - an identity when somebody signs in, activity when they connect a watch.

Four things have to be true of that project. The old single "OAuth consent screen" page no longer exists - it is now three pages under **Google Auth Platform**, which is why these are given as direct links.

1. The **Google Health API** is enabled: [API Library](https://console.cloud.google.com/apis/library/health.googleapis.com).
2. `<your API URL>/v1/devices/google-health/callback` is one of the OAuth client's authorized redirect URIs: [Credentials](https://console.cloud.google.com/apis/credentials). Signing in and connecting a watch come back to different routes, so adding this one is a step of its own.
3. The Google Health scope is listed on [Data Access](https://console.cloud.google.com/auth/scopes) - *Add or remove scopes*, search for Google Health API.
4. Whoever is testing is on the test-user list: [Audience](https://console.cloud.google.com/auth/audience), where *Publishing status* reads **Testing** and *User type* **External**. Add each address under **Test users**.

Two things to plan for while the app is in Testing:

- It is capped at **100 test users**, each added by hand.
- A test user's authorization **expires seven days after they consent**, so the connection drops weekly until the app is published. StrongFish says so plainly - "the connection has expired, connect the account again" - rather than showing an empty chart.

Both limits go away once the app is published and verified. In a Google Workspace organization, an admin marking the app **Trusted** lifts them for that organization's own users.

### Withings

1. Create an application in the [Withings developer dashboard](https://developer.withings.com/dashboard/).
2. Register `<your API URL>/v1/devices/withings/callback` as its callback URL.
3. Set `SF_WITHINGS_CLIENT_ID` and `SF_WITHINGS_CLIENT_SECRET`.

### Stance Fitness

The Stance integration runs on values that Stance provides. Email their team at **hello@stancefitness.co** to request access, then set what they send you:

- `SF_STANCE_SUPABASE_PROJECT_REF` and `SF_STANCE_SUPABASE_ANON_KEY` - the account and the stored sets. **Required**: with either unset, Stance shows as not enabled here, like any other integration without its credentials.
- `SF_STANCE_PREPROCESS_URL` - the service that turns a raw set into a measured RPE. Optional; left unset, StrongFish still shows the bar speed and the RPE you gave, just not a measured one.
- `SF_STANCE_APP_URL` - what the phone's **Record a set** button opens. Optional; defaults to the Stance app's store page.
