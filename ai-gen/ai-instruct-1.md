# AI instruct 1

Create a powerlifting app which is called "strong-fish".

The stack will be based on the one used by `~/cwclock`:
* PostgreSQL as database
* Flywaydb for database migrations (`sf-db` folder)
* golang for the API (`sf-api` folder)
* React for the web-frontend (`sf-ui` folder)
* Flutter for the mobile app (`sf-mobile` folder)

The app will have the following rules:
* everyone can subscribe (as `~/cwclock` I want to use the `cwcloud` email api with the righ environment variable)
* there's superadmin role and coach role

A coach can create "clubs" (which is similar to cwclock's organizations) and add or remove member. A member can become an admin of the club and the coach is owner of the club.

A coach or admin can upload programs in the club.
* Program is similar to [`./assets/program.xlsx`](./assets/program.xlsx)
* Fix the load calculation from RPE and 1RM field by each member
* Coach can upload an excel file like this and it will be automatically converted in the data-model
* Sometimes there's some set without RPE but with a load defined (calculated from percentage of the fill 1RM)

A member can update the 1RM of each exercice as many time as he want and it recalculate every set of he's program.

On each set the member can defined the real RPE he perceive during the set and add comment feedback for the coach.

# public profile of member/coach

There's a public profile of member and coach (with avatar)

I want also a social network feature similar to `~/uprodit`'s newspaper but with better datamodel based on `cwclock`'s data model (using as possible JSONB payloads).

Each people can be follow/unfollow comment or report.

Superadmin can edit or delete every post or comment.

A post can be visible only to a club or everyone.

Each post can be liked and I want also the same web-component for video player when there's a video link detected in `~/uprodit/prodit-ui`.

Picture can be uploaded and stored as b64 in the payload.

# Authentication

I want MFA with USB device as yubikey and TOPT apps like in `~/cwclock`.

I want OIDC optional variables for Google, Github and keycloack.
