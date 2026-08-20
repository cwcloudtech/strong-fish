#!/usr/bin/env bash
#
# Builds and signs the iOS app, and leaves the .ipa where the pipeline picks it
# up (sf-mobile/dist/).
#
# Run by hand on a Mac. Everything here needs Xcode, the iOS SDK and codesign,
# none of which exist on the Linux runner that builds everything else - and
# Apple's licence keeps macOS on Apple hardware, so there is no image to run it
# in either. This is the seam: the Linux pipeline does the rest, and picks up
# whatever this last left behind.
#
#   cp .env.ios.example .env.ios   # once, then fill it in
#   ./ci/app/deliver-ios.sh            # build and sign
#   ./ci/app/deliver-ios.sh --upload   # ...and send it to TestFlight
#
# The version comes from VERSION, the same file the Android build and the API's
# manifest read, so a release is one number in one place.
set -euo pipefail

cd "$(dirname "$0")/../.."

ENV_FILE="${ENV_FILE:-.env.ios}"
UPLOAD=false
[[ "${1:-}" == "--upload" ]] && UPLOAD=true

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "No ${ENV_FILE}. Copy .env.ios.example to ${ENV_FILE} and fill it in." >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a && source "${ENV_FILE}" && set +a

require() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "${name} is not set in ${ENV_FILE}." >&2
    exit 1
  fi
}

require IOS_TEAM_ID

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This has to run on a Mac: the iOS toolchain is macOS-only." >&2
  exit 1
fi

# --- tooling ----------------------------------------------------------------

# ensure_tool installs what is missing rather than stopping at the first gap.
#
# Homebrew is not installed for you: its installer is a script fetched from the
# network and run with sudo, and a build script is the wrong place to do that
# on somebody's behalf. Xcode is not installed either - it is an App Store
# download of many gigabytes, and only a person can accept its licence.
ensure_tool() {
  local command_name="$1" formula="$2" cask="${3:-}"

  if command -v "${command_name}" >/dev/null; then
    return
  fi
  if ! command -v brew >/dev/null; then
    echo "${command_name} is missing and Homebrew is not installed." >&2
    echo "Install Homebrew first: https://brew.sh" >&2
    exit 1
  fi

  echo "==> Installing ${formula} (${command_name} not found)"
  if [[ -n "${cask}" ]]; then
    brew install --cask "${formula}"
  else
    brew install "${formula}"
  fi

  command -v "${command_name}" >/dev/null || {
    echo "${command_name} is still not on PATH after installing ${formula}." >&2
    exit 1
  }
}

if ! command -v xcodebuild >/dev/null; then
  echo "Xcode is not installed. Get it from the App Store, then run:" >&2
  echo "  sudo xcode-select --switch /Applications/Xcode.app" >&2
  echo "  sudo xcodebuild -license accept" >&2
  exit 1
fi
# xcodebuild exists but points at the command line tools alone, which cannot
# build an app - a common state on a Mac where Xcode was installed later.
if ! xcodebuild -version >/dev/null 2>&1; then
  echo "xcodebuild is not usable. If Xcode is installed, point at it:" >&2
  echo "  sudo xcode-select --switch /Applications/Xcode.app" >&2
  exit 1
fi

ensure_tool flutter flutter cask
ensure_tool pod cocoapods

VERSION="$(grep -oE '^[0-9\.]+$' VERSION)"
# The same integer the Android build derives, so the two platforms stay on one
# numbering: 1.2.3 -> 10203. iOS wants CFBundleVersion to rise with every
# upload, and deriving it from VERSION means it does so without being tracked
# separately.
BUILD_NUMBER="$(echo "${VERSION}" | awk -F. '{printf "%d%02d%02d", $1, $2, $3}')"
IPA_NAME="strong-fish-v${VERSION}.ipa"
DIST_DIR="sf-mobile/dist"

echo "==> StrongFish ${VERSION} (build ${BUILD_NUMBER}) for iOS"

# --- signing material -------------------------------------------------------
#
# Written to a keychain of its own rather than the login one: this runs on
# somebody's laptop, and a script that unlocks a personal keychain to sign is a
# script that leaves it unlocked. The keychain is deleted on the way out,
# whether or not the build worked.
KEYCHAIN="strong-fish-ios.keychain-db"
KEYCHAIN_PASSWORD="$(uuidgen)"
# One directory for everything decoded out of .env.ios, removed on the way out
# - so a failed build does not leave a signing certificate in /tmp.
WORK_DIR="$(mktemp -d -t strong-fish-ios)"
PUBSPEC_BACKUP="${WORK_DIR}/pubspec.yaml.orig"
cp sf-mobile/pubspec.yaml "${PUBSPEC_BACKUP}"

cleanup() {
  security delete-keychain "${KEYCHAIN}" 2>/dev/null || true
  # The version is stamped into pubspec.yaml for the build and put back
  # afterwards: the only thing that should come out of this script is the
  # .ipa, and a working copy left carrying a version bump is how the wrong
  # thing gets committed alongside it.
  [[ -f "${PUBSPEC_BACKUP}" ]] && cp "${PUBSPEC_BACKUP}" sf-mobile/pubspec.yaml
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

# CERTIFICATE_FILE is the signing identity as a PKCS#12 bundle.
#
# Made once, on the first run, from the Apple Distribution identity Xcode has
# already put in this Mac's login keychain - and then reused. It is the thing
# that signs: every run imports it into a keychain of its own rather than
# reaching into the login keychain, so what is signed depends on this file and
# not on whatever else the machine has accumulated.
CERTIFICATE_FILE="ios.p12"

# create_certificate writes CERTIFICATE_FILE from an identity in the login
# keychain, and records the password it invented.
create_certificate() {
  local identity export_file

  # find-identity prints  1) <sha1> "Apple Distribution: Name (TEAMID)"
  # Distribution only: a development identity cannot sign a build for
  # TestFlight or the store, and picking one here would fail much later with a
  # far less obvious message.
  # `|| true` because no match is the case this function exists to explain:
  # under `set -e` with `pipefail`, a grep that matches nothing would abort the
  # script here and the message below - the one telling you to make a
  # certificate - would never print.
  identity="$(security find-identity -v -p codesigning |
    grep -E '"(Apple|iPhone) Distribution' |
    grep -F "(${IOS_TEAM_ID})" |
    head -n 1 |
    sed -E 's/.*"(.*)"/\1/' || true)"

  if [[ -z "${identity}" ]]; then
    echo "No Apple Distribution identity in your keychain for ${IOS_TEAM_ID}." >&2
    echo "Create one in Xcode: Settings > Accounts > your Apple ID >" >&2
    echo "Manage Certificates > + > Apple Distribution. Then run this again." >&2
    exit 1
  fi

  echo "==> Exporting \"${identity}\" to ${CERTIFICATE_FILE}"

  # The format requires a password. It is generated here and appended to
  # .env.ios, because a .p12 whose password nobody recorded is a file that has
  # to be made again from scratch - and the whole point is that this one stays.
  IOS_CERTIFICATE_PASSWORD="$(uuidgen)"
  printf '\n# Written by ci/app/deliver-ios.sh when it created %s. Delete both to start over.\nIOS_CERTIFICATE_PASSWORD=%s\n' \
    "${CERTIFICATE_FILE}" "${IOS_CERTIFICATE_PASSWORD}" >> "${ENV_FILE}"

  export_file="${WORK_DIR}/export.p12"
  # -t identities exports every code-signing identity in the login keychain,
  # not just the one matched above: `security export` has no way to select one.
  # What is in the file is listed afterwards so it is never a surprise.
  security export -k login.keychain-db -t identities -f pkcs12 \
    -P "${IOS_CERTIFICATE_PASSWORD}" -o "${export_file}"
  mv "${export_file}" "${CERTIFICATE_FILE}"
  chmod 600 "${CERTIFICATE_FILE}"

  echo "==> ${CERTIFICATE_FILE} created, and its password saved in ${ENV_FILE}."
  echo "    It contains:"
  openssl pkcs12 -in "${CERTIFICATE_FILE}" -passin "pass:${IOS_CERTIFICATE_PASSWORD}" \
    -nokeys 2>/dev/null |
    openssl x509 -noout -subject 2>/dev/null |
    sed 's/^/      /' ||
    echo "      (could not be listed - openssl could not read it back)"
  echo "    Both files are gitignored. Keep them off shared machines."
}

if [[ ! -f "${CERTIFICATE_FILE}" ]]; then
  create_certificate
elif [[ -z "${IOS_CERTIFICATE_PASSWORD:-}" ]]; then
  # The file is here but the password is not: nothing can open it, and there is
  # no way to recover one. Saying so beats an import failing on a wrong
  # password three lines further down.
  echo "${CERTIFICATE_FILE} exists but IOS_CERTIFICATE_PASSWORD is not in ${ENV_FILE}." >&2
  echo "Delete ${CERTIFICATE_FILE} and run this again to make a new one." >&2
  exit 1
else
  echo "==> Signing with ${CERTIFICATE_FILE}"
fi

# A keychain of its own, deleted on the way out. This runs on somebody's
# laptop, and a script that unlocks a personal keychain to sign is a script
# that leaves it unlocked.
security create-keychain -p "${KEYCHAIN_PASSWORD}" "${KEYCHAIN}"
security set-keychain-settings -lut 3600 "${KEYCHAIN}"
security unlock-keychain -p "${KEYCHAIN_PASSWORD}" "${KEYCHAIN}"
security import "${CERTIFICATE_FILE}" -k "${KEYCHAIN}" \
  -P "${IOS_CERTIFICATE_PASSWORD}" -T /usr/bin/codesign -T /usr/bin/security
# Without this, codesign stops to ask for permission - which never arrives in
# a script.
security set-key-partition-list -S apple-tool:,apple:,codesign: \
  -s -k "${KEYCHAIN_PASSWORD}" "${KEYCHAIN}" >/dev/null
# Prepended to the search list rather than replacing it, or codesign stops
# finding everything else this Mac has.
# shellcheck disable=SC2046 # the substitution is meant to word-split
security list-keychains -d user -s "${KEYCHAIN}" $(security list-keychains -d user | tr -d '"')

# --- build ------------------------------------------------------------------

pushd sf-mobile >/dev/null

# Same rewrite the Dockerfile does for Android, for the same reason: the
# version lives in VERSION, and pubspec is generated from it rather than being
# a second place to remember.
sed -i '' "s/^version: .*/version: ${VERSION}+${BUILD_NUMBER}/" pubspec.yaml

flutter pub get
cd ios && pod install --repo-update && cd ..

# A real file, not a process substitution: xcodebuild parses this with a plist
# reader that seeks, and a /dev/fd pipe is not seekable.
EXPORT_PLIST="${WORK_DIR}/ExportOptions.plist"
cat > "${EXPORT_PLIST}" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>method</key>
  <string>app-store-connect</string>
  <key>teamID</key>
  <string>${IOS_TEAM_ID}</string>
  <key>uploadSymbols</key>
  <true/>
  <key>signingStyle</key>
  <string>automatic</string>
</dict>
</plist>
PLIST

# The dart-defines say where the installed app looks for its update and its
# download. iOS never self-updates (see AppUpdateNotifier.selfUpdateAllowed),
# but the first is also the API everything else talks to, so it is passed
# exactly as the Android build passes it.
flutter build ipa --release \
  --dart-define=SF_UPDATE_URL="https://api.strong-fish.com" \
  --dart-define=SF_DOWNLOAD_URL="https://www.strong-fish.com" \
  --export-options-plist="${EXPORT_PLIST}"

popd >/dev/null

BUILT="$(find sf-mobile/build/ios/ipa -maxdepth 1 -name '*.ipa' | head -n 1)"
if [[ -z "${BUILT}" ]]; then
  echo "The build produced no .ipa." >&2
  exit 1
fi

mkdir -p "${DIST_DIR}"
# One file, replaced each time: the pipeline copies whatever is in dist/ into
# the nginx image, and a directory accumulating every past release would ship
# them all.
rm -f "${DIST_DIR}"/*.ipa
cp "${BUILT}" "${DIST_DIR}/${IPA_NAME}"

echo "==> ${DIST_DIR}/${IPA_NAME}"
echo "    Commit it and push: the pipeline adds it to the ui-and-mobile image."

# --- TestFlight -------------------------------------------------------------

if [[ "${UPLOAD}" == "true" ]]; then
  require IOS_API_KEY_ID
  require IOS_API_ISSUER_ID

  # The key stays as the .p8 Apple gave you, in the repository root, under the
  # name it was downloaded with. No base64: Apple lets you download it once, so
  # the file is the thing to keep, and keeping it is simpler than transcribing
  # it into an environment file.
  SOURCE_KEY="AuthKey_${IOS_API_KEY_ID}.p8"
  if [[ ! -f "${SOURCE_KEY}" ]]; then
    echo "${SOURCE_KEY} is not in the repository root." >&2
    echo "Download it from App Store Connect > Users and Access > Integrations," >&2
    echo "keep Apple's filename, and put it here. It is gitignored." >&2
    exit 1
  fi

  # altool looks for the key by name in one of a few fixed directories; this is
  # the one that needs no extra flags. Copied there for the upload and removed
  # afterwards, so the only lasting copy is the one in the repository root.
  KEYS_DIR="${HOME}/private_keys"
  mkdir -p "${KEYS_DIR}"
  API_KEY_FILE="${KEYS_DIR}/${SOURCE_KEY}"
  cp "${SOURCE_KEY}" "${API_KEY_FILE}"

  echo "==> Uploading to App Store Connect"
  xcrun altool --upload-app -f "${DIST_DIR}/${IPA_NAME}" -t ios \
    --apiKey "${IOS_API_KEY_ID}" --apiIssuer "${IOS_API_ISSUER_ID}"

  rm -f "${API_KEY_FILE}"
  echo "==> Uploaded. It appears in TestFlight once Apple finishes processing."
fi
