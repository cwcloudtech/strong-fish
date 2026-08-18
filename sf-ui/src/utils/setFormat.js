/**
 * How a prescribed set reads, shared by the authoring table, the training
 * table and the public program page so the three can't drift apart.
 */

/**
 * The rep target.
 *
 * Zero is not "no reps": it's a set the coach prescribed without a rep number
 * - the block spreadsheets write "3 x AMRAP" - so the count is unknown until
 * it's done. Printing "0" would read as a mistake, and the instruction itself
 * is already on the row as the set's note.
 */
export function formatReps(set) {
  return set?.reps > 0 ? String(set.reps) : "—";
}

/** A one-line summary of how a set is loaded. */
export function describeLoad(t, set) {
  switch (set.loadMode) {
    case "rpe":
      return `${t("session.rpe")} ${set.rpe ?? "—"}`;
    case "percentage":
      return `${set.percentage ?? "—"}%`;
    case "absolute":
      return `${set.absoluteLoad ?? "—"} ${t("common.kg")}`;
    default:
      return t("session.bodyweight");
  }
}
