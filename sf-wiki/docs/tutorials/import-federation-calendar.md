---
id: import-federation-calendar
title: Importing the federation calendar
sidebar_position: 4
---

The FFForce publishes each season as a PDF year planner. If you coach, you can
upload that file and StrongFish will read the competitions out of it — dates,
names and all — instead of you retyping forty entries.

## Where to get the file

The federation posts it on its own site, under
[Compétitions Force Athlétique National](https://www.ffforce.fr/fr/force-athletique-ffforce/national-force-athletique/competitions-force-athletique-national/saison-2026-national-fa.html).
Download the season's PDF from that page — the whole-year planner with the
months across the top, not a single competition's entry form.

**Who can do this:** a coach, for their own club, and a superadmin, who can also
put a season on the open calendar everyone sees. It is the same permission as
adding one event by hand, because that is exactly what an import is — many of
them at once.

## Uploading it

1. Open **Events**.
2. Click **Import a calendar**.
3. Pick the club the season belongs to. A superadmin can leave this blank to
   publish to the open calendar instead.
4. Choose the PDF.

It reads in a second or two, and the competitions appear on your calendar
straight away.

## What comes across

**Every competition becomes an all-day event.** The planner records days, never
a time of day: a national meet is written into the grid as "13–15/05", and that
is what it means. Nothing invents a 9 a.m. start.

**The colours come with them.** On the printed page, the category — federal,
European, world, special meeting — is shown by the colour a competition is
shaded in, and by nothing else. StrongFish keeps that colour on the event, so a
month of imported dates stays as readable on screen as it was on paper.

**Dates come from what is written, first.** Where a competition carries its
dates in the label, those are used, in whatever form they were typed:

| On the calendar | Read as |
| --- | --- |
| `13/05 - 15/05` | 13 to 15 May |
| `13-05 / 15-05` | 13 to 15 May |
| `13-15/05` | 13 to 15 May |
| `24 au 27/07` | 24 to 27 July |
| `30 au 1er Nov` | 30 October to 1 November |
| `9-15` | the 9th to the 15th of that column's month |

Where an entry has no date written on it, the coloured band beside it is used
instead — the days it covers in its own month column.

## Re-importing a revised calendar

The federation revises the season through the year and republishes it. Upload
the new file the same way: anything already on your calendar is left alone, and
only what changed is added. The result tells you how many were added and how
many were already there.

## Entries it could not date

A planner is a human document. A few entries carry no date and no shading —
a note in the margin, a deadline written sideways. Those are listed by name
after the import, so you can add them by hand rather than discovering the gap
in March.

If the whole file is refused, check you downloaded the year planner and not a
scanned image of one: the import reads the text and the shapes in the PDF, so a
photograph of a calendar has nothing in it to read.
