# strong-fish

![strong-fish](./img/logo.png)

A powerlifting app: RPE-based programs whose loads are computed from each
athlete's own current 1RM, clubs to coach them in, and a training feed.

## Features

* **Programs from a spreadsheet.** A coach uploads the `.xlsx` they already
  write their blocks in and it becomes a real program - weeks, sessions and
  every prescribed set (see [Importing a program](#importing-a-program)).
* **Loads that follow the athlete.** Nothing derived is ever stored: every set's
  weight is computed from the member's own current max at read time, so updating
  one 1RM recalculates the whole program (see [Load calculation](#load-calculation)).
* **RPE feedback.** On each set the member logs the RPE they actually felt, the
  weight they used and a comment for their coach, and gets the estimated 1RM
  that performance demonstrates.
* **Clubs.** A coach owns the clubs they create, promotes members to admin, and
  assigns programs. Coaches read their club's feedback in one inbox.
* **A shared exercise catalog.** A coach adds "larsen press" once and every other
  coach gets it in their autocomplete, with English and French labels.
* **Public profiles and a feed.** Follow, post, like, comment and report;
  posts are public or club-only, carry inline pictures, and embed a detected
  video link with a player.
* **Authentication.** Anyone can sign up. MFA with TOTP authenticator apps and
  WebAuthn security keys (YubiKey and friends). Optional OIDC login with Google,
  GitHub and Keycloak.
* **I18N** in English and French, across the API's error codes, the web app and
  the mobile app.
* Light/dark mode.

## Technologies

* [Go](https://go.dev) and [PostgreSQL](https://www.postgresql.org) for the API
  ([`sf-api`](./sf-api))
* [Flyway](https://flywaydb.org) for the migrations ([`sf-db`](./sf-db))
* [React](https://react.dev) for the web app ([`sf-ui`](./sf-ui))
* [Flutter](https://flutter.dev) for the mobile app ([`sf-mobile`](./sf-mobile))

## Getting started

The whole stack (PostgreSQL, the Flyway migrations, the API and the web app
behind nginx) starts with one command:

```shell
docker compose up --build
```

Then open [`http://localhost:3000`](http://localhost:3000). **The first account
you register becomes the superadmin**; every later one is disabled until it's
activated - by the link emailed to it (`SF_ACTIVATION_MODE=email`, the default)
or by a superadmin (`SF_ACTIVATION_MODE=admin`).

Copy [`.env.example`](./.env.example) to configure a real deployment. Outgoing
email goes through CWCloud's email API, driven by `CWCLOUD_API_URL` and
`CWCLOUD_API_KEY`; with those unset, mail is logged and skipped rather than
failing a registration.

### Running the pieces separately

```shell
cd sf-api && go run .
cd sf-ui && npm install && npm start
cd sf-mobile && flutter run
```

## Load calculation

This is the core of the app, and the thing the reference spreadsheet
([`ai-gen/assets/program.xlsx`](./ai-gen/assets/program.xlsx)) gets wrong.

Powerlifting programs are written in RPE: "3 reps @ RPE 8" means three reps
leaving two in reserve. How much of a one-rep max that represents is read off
the standard RPE chart.

The spreadsheet doesn't compute its loads that way. Every row carries a
`Percentage` column typed in by hand from that chart, and the load is
`percentage/100 × 1RM`. Two things go wrong:

1. **The typed percentages contradict each other.** Across the five weeks, eight
   distinct (reps, RPE) pairs are given two different percentages - 5 reps @
   RPE 8 is 82% in one session and 78% in another; 3 reps @ RPE 8 is 87% in one
   and 81% in another. Whichever was meant, both can't be right.
2. **They're frozen against the author's own maxes.** A percentage doesn't follow
   the member actually running the program, and doesn't move when they get
   stronger.

[`internal/loadcalc`](./sf-api/internal/loadcalc) fixes both by reading the chart
directly against the member's own current 1RM:

```
load = 1RM × Intensity(reps, RPE)
```

which is self-consistent - a set performed as prescribed estimates back to
exactly the max it was prescribed from, where the spreadsheet's own `e1RM` column
drifts by up to 3kg - and recomputes for free whenever the member updates a max,
since no derived weight is ever stored.

Three other prescription kinds are supported, because the source file uses all of
them: a fixed **percentage** of the member's 1RM (for sets the coach deliberately
left without an RPE - the spreadsheet writes those as `?`), an **absolute** weight
for accessories, and **bodyweight** movements like pull-ups and dips.

Loads are also reported snapped to a loadable increment (`SF_PLATE_INCREMENT`,
2.5kg by default - one small plate per side).

## Importing a program

`POST /v1/clubs/{clubId}/programs/import` takes the coach's `.xlsx`. The expected
shape is the reference file's: one sheet per week, a `refs` sheet holding the
1RMs the percentages were authored against, and each training day starting with
an `Exercice | Reps | RPE | Percentage | Load | e1RM | Part` header row.

The importer is deliberately tolerant, because real files aren't clean - in the
reference file four day-title cells were clobbered by a stray fill-down formula
and evaluate to an exercise name or a bare number. A day is therefore recognized
by its header row rather than its title, and week/day numbering falls back to the
sheet name plus the block's position. Anything it had to guess at comes back in
the response's `warnings` so the coach sees it.

It also recovers what the file never states outright: which competition lift each
movement is programmed off. A Larsen press row just computes "82% of `refs!B4`",
so the reference lift is inferred from the arithmetic - the row's cached load and
percentage imply the max the author used, and that lands on one of the reference
1RMs. Movements the catalog doesn't know yet are added to it, so they're
available to every other coach from then on.

## Repository layout

| Path | What's in it |
| --- | --- |
| [`sf-db`](./sf-db) | Flyway migrations: the schema, and the exercise catalog seeded from the reference program |
| [`sf-api`](./sf-api) | The Go API |
| [`sf-ui`](./sf-ui) | The React web app |
| [`sf-mobile`](./sf-mobile) | The Flutter mobile app |
| [`ai-gen`](./ai-gen) | The instructions and assets this was built from |

Every table is a thin set of indexed, foreign-keyed columns plus a single `data`
JSONB payload, so the schema stays stable while the domain grows.

## Tests

```shell
cd sf-api && go test ./...
cd sf-ui && CI=true npm run build
cd sf-mobile && flutter analyze
```

The API's tests cover the load calculation against the reference spreadsheet's
own numbers, the spreadsheet importer against the real file, and the router (a
subrouter silently shadowing a route is not a startup error in chi, so every
endpoint is asserted to resolve).

## The i18n dictionaries

The web app's dictionaries are the source of truth; the mobile app's Dart
dictionaries are generated from them so a string added once works in both:

```shell
cd sf-ui && node tools/gen_dart_i18n.mjs
```
