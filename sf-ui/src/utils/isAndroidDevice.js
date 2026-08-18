/**
 * Whether this browser is running on Android, which decides what the "get the
 * app" control does: follow the download link directly, or - on a device that
 * can't install an APK - open a QR code to scan with a phone that can.
 */
export default function isAndroidDevice() {
  return /Android/i.test(navigator.userAgent || "");
}
