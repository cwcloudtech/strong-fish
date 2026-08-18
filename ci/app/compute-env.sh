#!/usr/bin/env bash

export APPS="ui api ui-and-mobile wiki"
export APP_PREFIX="sf"
export VERSION="$(grep -oE "^[0-9\.]+$" VERSION)"
export VERSION_SHA="${VERSION}-${CI_COMMIT_SHORT_SHA}"
export SF_API_URL="https://api.strong-fish.com"
export SF_UI_URL="https://www.strong-fish.com"
export SF_DOC_URL="https://doc.strong-fish.com"
export SF_ABOUT_URL="https://doc.strong-fish.com/docs/about"
export SF_CORS_ENABLED="off"
export SF_MAX_IMAGE_SIZE="2097152"
export SF_ACTIVATION_MODE="email"

echo "VERSION=${VERSION}" > .env.ci
echo "VERSION_SHA=${VERSION_SHA}" >> .env.ci

# Mobile release-signing key (ai-instruct-128). MOBILE_KEYSTORE_BASE64 /
# MOBILE_KEYSTORE_PASSWORD / MOBILE_KEY_ALIAS / MOBILE_KEY_PASSWORD are
# GitLab CI/CD variables (masked, protected on main). Decoded to a file here
# because docker-compose-build.yml's build secret needs a path on disk;
# MOBILE_KEYSTORE_PASSWORD/MOBILE_KEY_ALIAS/MOBILE_KEY_PASSWORD are already
# plain env vars, so compose reads them directly. When the variable isn't
# set (e.g. a fork or local run), MOBILE_KEYSTORE_FILE stays unset and
# docker-compose-build.yml's default (/dev/null) keeps the build working
# with the debug-signing fallback.
if [[ -n "${MOBILE_KEYSTORE_BASE64:-}" ]]; then
  export MOBILE_KEYSTORE_FILE="$(pwd)/mobile-release.keystore"
  echo "${MOBILE_KEYSTORE_BASE64}" | base64 -d > "${MOBILE_KEYSTORE_FILE}"
fi
# docker-compose-build.yml sources these three straight from the
# environment (no `:-default` fallback available for compose secrets), so
# they must always be at least an empty string for local/fork bake runs
# where the real CI/CD variables aren't set.
export MOBILE_KEYSTORE_PASSWORD="${MOBILE_KEYSTORE_PASSWORD:-}"
export MOBILE_KEY_ALIAS="${MOBILE_KEY_ALIAS:-}"
export MOBILE_KEY_PASSWORD="${MOBILE_KEYSTORE_PASSWORD}"
