/** Default cap, matching the API's own when /v1/config didn't report one. */
const DEFAULT_MAX_IMAGE_SIZE = 2 * 1024 * 1024;

/**
 * Reads a picked image into a base64 data URI, which is how avatars, club logos
 * and post pictures are carried and stored (inline in the JSONB payload - there
 * is no object store).
 *
 * Oversized files are rejected here rather than being sent and refused: base64
 * inflates a file by a third, so a client-side check saves a wasted round trip
 * of exactly the payload that was too big.
 */
export function readImageAsDataUrl(file, maxBytes = DEFAULT_MAX_IMAGE_SIZE) {
  return new Promise((resolve, reject) => {
    if (file.size > (maxBytes || DEFAULT_MAX_IMAGE_SIZE)) {
      reject(new Error("too-large"));
      return;
    }
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(new Error("unreadable"));
    reader.readAsDataURL(file);
  });
}
