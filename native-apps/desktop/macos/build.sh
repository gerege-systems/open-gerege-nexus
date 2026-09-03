#!/usr/bin/env bash
# Gerege Nexus — macOS клиентийг барих.
#
# CI энэ файлыг дуудна (.github/workflows/native-clients.yml). Xcode төслийг
# репод хадгалдаггүй: `project.yml` нь эх сурвалж, `.xcodeproj` нь артефакт
# (`.gitignore`). Тиймээс барихын өмнө үргэлж дахин үүсгэнэ — эс бөгөөс шинэ
# файл нэмсэн хүн «яагаад компайл хийгдэхгүй байна» гэдгийг олоход хагас
# өдөр алдана.
#
#   ./build.sh                 # Debug
#   CONFIGURATION=Release ./build.sh
#
# Шаардлага: Xcode 16+, `brew install xcodegen`. `../gerege-token-kit`
# (локал SPM багц) автоматаар resolve хийгдэнэ.

set -euo pipefail

cd "$(dirname "$0")"

CONFIGURATION="${CONFIGURATION:-Debug}"
SCHEME="${SCHEME:-NexusGeregeDesktop}"

command -v xcodegen >/dev/null || { echo "xcodegen олдсонгүй: brew install xcodegen" >&2; exit 1; }

xcodegen generate

# CODE_SIGNING_ALLOWED=NO нь CI-д зориулагдсан: тэнд гарын үсгийн түлхүүр
# байхгүй бөгөөд энэ шалгалтын асуулт нь «компайл хийгдэж байна уу», «түгээхэд
# бэлэн үү» биш.
xcodebuild -project "${SCHEME}.xcodeproj" -scheme "$SCHEME" \
           -configuration "$CONFIGURATION" \
           -destination 'platform=macOS' \
           CODE_SIGNING_ALLOWED=NO \
           build
