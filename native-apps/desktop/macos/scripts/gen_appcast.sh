#!/usr/bin/env bash
# Generate a Sparkle appcast.xml for a single DMG release.
#
# Usage:
#   scripts/gen_appcast.sh <dmg-path> [marketing-version] [build-number] [notes-url]
#
# Marketing version (CFBundleShortVersionString) and build (CFBundleVersion) are
# read straight from the .app inside the DMG so the appcast can never disagree
# with the shipped binary — Sparkle compares `sparkle:version` against the
# running app's CFBundleVersion, so a mismatch breaks update detection. Pass them
# explicitly only to override.
#
# The DMG is signed with Sparkle's EdDSA key — from the keychain
# ("https://sparkle-project.org", the `generate_keys` default) or, in CI, from
# the SPARKLE_ED_PRIVATE_KEY env var (base64 key exported via `generate_keys -x`).
# A missing/invalid signature makes Sparkle silently refuse the update.
#
# `sign_update` is located from the resolved Sparkle SwiftPM artifacts unless
# SIGN_UPDATE points at it explicitly. Emits the appcast XML to stdout.
set -euo pipefail

DMG="${1:?usage: gen_appcast.sh <dmg> [marketing-version] [build-number] [notes-url]}"
SHORT_VERSION="${2:-}"
BUILD="${3:-}"
NOTES_URL="${4:-}"

FEED_URL="https://e-id.mn/download/appcast.xml"
DOWNLOAD_URL="https://e-id.mn/download/$(basename "$DMG")"
MIN_SYSTEM="14.0"

[ -f "$DMG" ] || { echo "DMG not found: $DMG" >&2; exit 1; }

# Read versions from the .app inside the DMG unless overridden.
if [ -z "$SHORT_VERSION" ] || [ -z "$BUILD" ]; then
  MNT="$(mktemp -d)"
  hdiutil attach "$DMG" -nobrowse -readonly -mountpoint "$MNT" >/dev/null
  trap 'hdiutil detach "$MNT" >/dev/null 2>&1 || true; rmdir "$MNT" 2>/dev/null || true' EXIT
  APP="$(find "$MNT" -maxdepth 1 -name '*.app' | head -1)"
  [ -n "$APP" ] || { echo "no .app found inside $DMG" >&2; exit 1; }
  PLIST="$APP/Contents/Info.plist"
  [ -z "$SHORT_VERSION" ] && SHORT_VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$PLIST")"
  [ -z "$BUILD" ] && BUILD="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$PLIST")"
fi

# Locate sign_update (SwiftPM artifact bundle, then PATH).
if [ -z "${SIGN_UPDATE:-}" ]; then
  SIGN_UPDATE="$(find "${HOME}/Library/Developer/Xcode/DerivedData" \
      -type f -name sign_update -path '*artifacts/sparkle*' 2>/dev/null | head -1 || true)"
fi
[ -z "${SIGN_UPDATE:-}" ] && SIGN_UPDATE="$(command -v sign_update || true)"
[ -n "${SIGN_UPDATE:-}" ] || { echo "sign_update tool not found (set SIGN_UPDATE)" >&2; exit 1; }

# Sign. In CI write the exported private key to a temp file and pass it via
# --ed-key-file (sign_update no longer accepts the key as an inline argument);
# locally fall back to the key stored in the login keychain.
if [ -n "${SPARKLE_ED_PRIVATE_KEY:-}" ]; then
  KEYFILE="$(mktemp)"
  printf '%s' "$SPARKLE_ED_PRIVATE_KEY" > "$KEYFILE"
  SIG_ATTRS="$("$SIGN_UPDATE" --ed-key-file "$KEYFILE" "$DMG")"
  rm -f "$KEYFILE"
else
  SIG_ATTRS="$("$SIGN_UPDATE" "$DMG")"
fi
# SIG_ATTRS looks like: sparkle:edSignature="…" length="12345"

PUB_DATE="$(LC_ALL=C date -u '+%a, %d %b %Y %H:%M:%S +0000')"
NOTES_LINK=""
[ -n "$NOTES_URL" ] && NOTES_LINK="            <sparkle:releaseNotesLink>${NOTES_URL}</sparkle:releaseNotesLink>"

cat <<XML
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle" xmlns:dc="http://purl.org/dc/elements/1.1/">
    <channel>
        <title>Gerege Nexus</title>
        <link>${FEED_URL}</link>
        <description>Gerege Nexus desktop updates</description>
        <language>mn</language>
        <item>
            <title>${SHORT_VERSION}</title>
${NOTES_LINK}
            <pubDate>${PUB_DATE}</pubDate>
            <sparkle:version>${BUILD}</sparkle:version>
            <sparkle:shortVersionString>${SHORT_VERSION}</sparkle:shortVersionString>
            <sparkle:minimumSystemVersion>${MIN_SYSTEM}</sparkle:minimumSystemVersion>
            <enclosure url="${DOWNLOAD_URL}" type="application/x-apple-diskimage" ${SIG_ATTRS} />
        </item>
    </channel>
</rss>
XML
