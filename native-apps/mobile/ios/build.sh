#!/usr/bin/env bash
# Gerege Nexus — iOS/iPadOS клиентийг барих.
#
# Ширээнийхтэй ижил дүрэм: `project.yml` нь ЭХ СУРВАЛЖ, `.xcodeproj` нь артефакт
# (`.gitignore`). Тиймээс барихын өмнө үргэлж дахин үүсгэнэ — шинэ файл нэмсэн
# хүн «яагаад компайл хийгдэхгүй байна» гэдгийг олоход хагас өдөр алдахгүй.
#
#   ./build.sh                                  # Simulator, Debug
#   DESTINATION='generic/platform=iOS' ./build.sh
#
# Шаардлага: Xcode 16+, `brew install xcodegen`.
set -euo pipefail
cd "$(dirname "$0")"

SCHEME="${SCHEME:-NexusGeregeMobile}"
CONFIGURATION="${CONFIGURATION:-Debug}"
DESTINATION="${DESTINATION:-generic/platform=iOS Simulator}"

command -v xcodegen >/dev/null || { echo "xcodegen олдсонгүй: brew install xcodegen" >&2; exit 1; }
xcodegen generate

xcodebuild -quiet \
  -project NexusGeregeMobile.xcodeproj \
  -scheme "$SCHEME" \
  -configuration "$CONFIGURATION" \
  -destination "$DESTINATION" \
  CODE_SIGNING_ALLOWED=NO \
  build

echo "OK: $SCHEME ($CONFIGURATION) баригдлаа."
