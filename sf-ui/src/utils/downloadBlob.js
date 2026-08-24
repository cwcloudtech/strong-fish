/**
 * Hands a fetched file to the browser as a download.
 *
 * The exports are fetched rather than linked, so that a failure surfaces on the
 * page instead of replacing it with the API's error, and so the file is named
 * after the program rather than after the route. That means turning the blob
 * into a click here.
 */
export function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  // Revoked once the click has been handled, or the blob is held for the life
  // of the tab.
  URL.revokeObjectURL(url);
}

export default downloadBlob;
