import { useEffect, useState } from "react";

/**
 * The running app's version, for the sidebar badge.
 *
 * It comes from the repository's `manifest.json`, copied next to the built
 * frontend at deploy time (see the Dockerfile) rather than served by the API -
 * so it is fetched from the app's own origin, not REACT_APP_APIURL, and is
 * whatever was actually deployed rather than whatever was compiled in.
 *
 * The file is deployed as `manifest-version.json` because `manifest.json` is
 * already taken by the PWA manifest, which has no version field. The API's own
 * `/v1/config` reports the same number and is the fallback for a dev server,
 * where nothing has copied the file next to the assets yet.
 */
export default function useAppVersion(fallback) {
  const [version, setVersion] = useState(fallback || null);

  useEffect(() => {
    let cancelled = false;
    fetch("/manifest-version.json", { headers: { Accept: "application/json" } })
      .then((response) => (response.ok ? response.json() : null))
      .then((data) => {
        if (!cancelled && data?.version) setVersion(data.version);
      })
      .catch(() => {
        // Nothing to do: the fallback is already in state.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (fallback) setVersion((current) => current || fallback);
  }, [fallback]);

  return version;
}
