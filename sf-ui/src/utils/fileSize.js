/**
 * A byte count as something a person can compare a file against - "200 MB".
 *
 * Rounded rather than exact: the number exists to answer "is my video near the
 * limit", and 209715200 answers that worse than 200 MB does.
 */
export function formatBytes(bytes) {
  const mb = Number(bytes) / (1024 * 1024);
  if (!Number.isFinite(mb) || mb <= 0) return "";
  return mb >= 1024 ? `${Math.round((mb / 1024) * 10) / 10} GB` : `${Math.round(mb)} MB`;
}

export default formatBytes;
