# AI instruct 9

## Profile

A member can choose visibility of its profile:
* public (everyone can see it without authentication)
* only clubs (everyone in the same club can see it without authentication)
* private (only superuser, or coach/admin from the clubs he joined can see it without authentication)

I want a search engine like `~/uprodit` which take this into account (search by email, name or surname).

Add also birthdate optional. If it's filled, automatically there's an event in the calendar of people who can see it.

## Invitation into a club

A coach or admin can invite everybody into his clubs.
An invitation link is sent by email or can be activated with a todo in the frontend or mobile invitations item.

# Coach confirmation

When signing up, the futur member can precise "I'm a coach" or "I'm an athlete".
If he choose "I'm a coach" it has to send notification to superuser to confirm he's really a coach or reject with a motive.
