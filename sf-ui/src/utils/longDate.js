/**
 * A date the way a credit line reads it: "October 19th of 2026",
 * "19 octobre 2026".
 *
 * Long rather than the numeric form the rest of the app uses for timestamps.
 * This one sits under a piece of writing, where "19/10/2026" reads as a
 * reference number and a month's name reads as a date.
 *
 * English takes the ordinal and the "of", which is how the line was asked
 * for; every other language gets its own long form from Intl, which knows
 * where the day goes.
 */
export default function longDate(locale, value) {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return "";

  if ((locale || "en").startsWith("en")) {
    const month = date.toLocaleDateString("en", { month: "long" });
    return `${month} ${ordinal(date.getDate())} of ${date.getFullYear()}`;
  }

  return date.toLocaleDateString(locale, { day: "numeric", month: "long", year: "numeric" });
}

/** 1st, 2nd, 3rd, 4th - and the three teens that look like exceptions and are not. */
function ordinal(day) {
  if (day < 11 || day > 13) {
    if (day % 10 === 1) return `${day}st`;
    if (day % 10 === 2) return `${day}nd`;
    if (day % 10 === 3) return `${day}rd`;
  }
  return `${day}th`;
}
