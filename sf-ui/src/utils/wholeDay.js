/**
 * Whether a calendar entry occupies days rather than a time of day.
 *
 * Mirrors the API's models.Event.WholeDay: either its author marked it all-day,
 * or it is a birthday - the one whole-day entry nobody authors, derived from a
 * birthdate that carries no time to show.
 *
 * One function rather than the check repeated per view, because the month grid,
 * the week grid and the agenda all have to agree: an entry drawn in the week's
 * all-day row must not also be drawn as an hour-long block below it.
 */
export default function isWholeDay(event) {
  return Boolean(event?.allDay) || event?.kind === "birthday";
}
