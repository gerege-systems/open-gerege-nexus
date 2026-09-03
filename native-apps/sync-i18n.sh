#!/usr/bin/env bash
#
# Нэвтрэх дэлгэцийн орчуулгыг цөмөөс дахин экспортлох — ЭНЭ БРЭНДИЙН нэрээр.
#
# Орчуулгууд нь цөмийн frontend-ийн `i18n:export-native`-ийн гаралт бөгөөд энэ
# репо frontend барьдаггүй. Экспортлогч нь толь бичгийн `{brand}` тэмдгийг
# ЭКСПОРТЫН МӨЧИД задалдаг — өөрөөр хэлбэл BRAND_NAME тавихаа мартвал долоон
# хэл дээрх хоёр мөр «Gerege Nexus» гэж шатаж, native клиент нь өөр
# бүтээгдэхүүний нэрийг иргэнд үзүүлнэ. Тэр яг нэг удаа болсон.
#
# Тиймээс гараар бүү экспортол — үүнийг ажиллуул:
#
#   native-apps/sync-i18n.sh          # цөм нь ЭНЭ репо
#
# Экспортлогч дөрвөн хэлбэрийг бичдэг ч энэ репод ГУРАВ нь л байна:
# generated-i18n/*.json, Android-ийн res/values*/auth.xml, iOS-ийн
# Login.xcstrings. Windows-ийн Login*.resx нь ХУУЧИН WPF бүрхүүлийнх байсан —
# ширээний клиент нь одоо eID-ийн бүтэн апп бөгөөд өөрийн
# Strings/<culture>/Resources.resw-тэй (`desktop/macos/scripts/gen_l10n.py` нь
# macOS-ийн каталогийг ТЭДГЭЭРЭЭС үүсгэдэг). Тиймээс тэдгээр мөр «алгасав»
# гэж гарах нь хэвийн, дутуу биш.
#
# Экспортлогч цөмийн repo дотор бичдэг тул энд хуулж авна.

set -euo pipefail

CORE="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
BRAND="${BRAND_NAME:-Gerege Nexus}"

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE="$(cd "$CORE" && pwd)" || { echo "цөмийн репо олдсонгүй: $1" >&2; exit 1; }
[ -f "$CORE/frontend/scripts/export-native-auth-i18n.mjs" ] || {
    echo "$CORE нь цөмийн репо биш байна (export-native-auth-i18n.mjs алга)" >&2; exit 1; }
command -v node >/dev/null || { echo "node олдсонгүй" >&2; exit 1; }

# Цөмийн ажлын мод хөндөгдөхгүй: экспортлогч нь гаралтаа cwd-ээс хамааруулан
# `../native-apps` руу бичдэг тул хэрэгтэй файлуудыг түр хавтас руу хуулж,
# тэндээс ажиллуулна. Цөм рүү шууд бичих нь өөр хүний ажлын модыг бохирдуулна.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/frontend/lib/i18n/addons" "$tmp/native-apps"
cp -R "$CORE/frontend/scripts" "$tmp/frontend/scripts"
cp "$CORE/frontend/lib/i18n/addons/auth.ts" "$tmp/frontend/lib/i18n/addons/"
# Экспортлогч нь mn ба en-ийг auth.ts-ээс, бусдыг locale бүрийн core.ts-ээс авна.
for l in ar zh fr ru es; do
    mkdir -p "$tmp/frontend/lib/i18n/locales/$l"
    cp "$CORE/frontend/lib/i18n/locales/$l/core.ts" "$tmp/frontend/lib/i18n/locales/$l/"
done

( cd "$tmp/frontend" && BRAND_NAME="$BRAND" node scripts/export-native-auth-i18n.mjs )

# Гаралт нь дөрвөн газарт. Байгаа файлыг л дарж бичнэ — экспортлогч шинэ зам
# нэмбэл энэ жагсаалт түүнийг алгасах бөгөөд тэр нь чимээгүй байхаас дээр:
# доорх тоолол хэдэн файл шинэчлэгдсэнийг хэлнэ.
n=0
while IFS= read -r src; do
    rel="${src#$tmp/native-apps/}"
    dst="$HERE/$rel"
    [ -f "$dst" ] || { echo "  алгасав (энэ репод алга): $rel"; continue; }
    cp "$src" "$dst"
    n=$((n + 1))
done < <(find "$tmp/native-apps" -type f)

echo "$n файл шинэчлэгдэв, брэнд: $BRAND"

# Шалгалт: экспортын дараа цөмийн анхдагч нэр үлдэх ёсгүй. Үлдсэн бол
# BRAND_NAME задраагүй гэсэн үг — тэр нь яг энэ скрипт байхаас сэргийлдэг зүйл.
if grep -rq "Gerege Nexus" "$HERE/generated-i18n" \
        "$HERE/mobile/android/app/src/main/res" \
        "$HERE/mobile/ios/Sources/GeregeShellUI/Resources" 2>/dev/null; then
    echo "АЛДАА: экспортын дараа «Gerege Nexus» үлдлээ — BRAND_NAME задраагүй байна" >&2
    exit 1
fi
echo "OK: үүсгэсэн мөрүүдэд цөмийн анхдагч нэр алга"
