#!/bin/bash
# Ажиглалтын домэйныг Gerege болгох: landing хуудас, Grafana-ийн нэвтрэх карт,
# лого, tab icon, нэр, ба Монгол хэл.
#
# Grafana OSS-д брэндлэх цэг байхгүй — white labeling нь Enterprise-д. Тиймээс
# бүх зүйл nginx дээр хийгдэнэ: хэв маягийг HTML-д шахаж оруулах, хоёр webpack
# chunk-ийг орлуулах, лого/icon-ыг өөр файлаар өгөх. Grafana-ийн контейнер
# өөрөө хөндөгдөхгүй — тиймээс шинэчлэлт, дахин суулгалт энэ ажлыг устгахгүй.
#
# Скрипт нь ИДЕМПОТЕНТ: Grafana шинэчлэгдэх бүрд дахин ажиллуулна. chunk-ийн
# нэрс (тэдгээрийн доторх hash) хувилбар бүрт өөрчлөгддөг тул тэдгээрийг
# ажиллаж буй контейнероос уншиж, nginx-ийн snippet-ийг дахин бичдэг.
#
# Хэрэглээ:
#   deploy/scripts/setup_monitor_branding.sh                      # nexus
#   MONITOR_DOMAIN=monitor.example.mn PLATFORM_HOST=example.mn \
#   DOCS_HOST=docs.example.mn deploy/scripts/setup_monitor_branding.sh
#
# Шаардлага: docker, python3, nginx, ажиллаж буй Grafana контейнер.

set -euo pipefail

MONITOR_DOMAIN="${MONITOR_DOMAIN:-monitor.nexus.gerege.mn}"
PLATFORM_HOST="${PLATFORM_HOST:-nexus.gerege.mn}"
DOCS_HOST="${DOCS_HOST:-docs.nexus.gerege.mn}"
GRAFANA_CONTAINER="${GRAFANA_CONTAINER:-gerege_nexus_grafana}"
WEBROOT="${WEBROOT:-/var/www/monitor}"
SNIPPET="${SNIPPET:-/etc/nginx/snippets/monitor-grafana-overrides.conf}"
RELOAD="${RELOAD:-1}"

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$DEPLOY_DIR/monitoring/grafana/branding"
LANDING_SRC="$DEPLOY_DIR/nginx/monitor-landing/index.html"

die() { echo "АЛДАА: $*" >&2; exit 1; }
say() { echo "  $*"; }

for tool in docker python3 nginx curl; do
  command -v "$tool" >/dev/null || die "$tool олдсонгүй"
done
docker ps --format '{{.Names}}' | grep -qx "$GRAFANA_CONTAINER" \
  || die "$GRAFANA_CONTAINER контейнер ажиллахгүй байна"
[ -f "$LANDING_SRC" ] || die "$LANDING_SRC олдсонгүй"

gexec() { docker exec "$GRAFANA_CONTAINER" sh -c "$1"; }
BUILD=/usr/share/grafana/public/build

echo "==> Grafana-ийн chunk-уудыг олж байна ($GRAFANA_CONTAINER)"

# Хэлний жагсаалт ба Branding-ийн мөрүүд нэг chunk дотор сууна.
LANG_CHUNK="$(gexec "grep -rlo 'AppTitle=\"Grafana\"' $BUILD/*.js | head -1 | xargs -r basename")"
[ -n "$LANG_CHUNK" ] || die "Branding chunk олдсонгүй — Grafana-ийн бүтэц өөрчлөгдсөн байж магадгүй"
say "branding + хэлний жагсаалт: $LANG_CHUNK"

# sv-SE-ийн орчуулгын chunk. app.js доторх зурганд:
#   "./sv-SE/grafana.json":[<module id>,[<chunk id>]]
IDS="$(gexec "grep -o '\"./sv-SE/grafana.json\":\[[0-9]*,\[[0-9]*\]\]' $BUILD/app.*.js | head -1")"
[ -n "$IDS" ] || die "sv-SE-ийн орчуулгын chunk олдсонгүй"
MODULE_ID="$(echo "$IDS" | sed 's/.*:\[\([0-9]*\),.*/\1/')"
CHUNK_ID="$(echo "$IDS" | sed 's/.*,\[\([0-9]*\)\]\]/\1/')"
MN_CHUNK="$(gexec "ls $BUILD | grep '^${CHUNK_ID}\.' | grep -v '\.map$' | head -1")"
[ -n "$MN_CHUNK" ] || die "chunk файл олдсонгүй: ${CHUNK_ID}.*"
say "sv-SE орчуулга: $MN_CHUNK (module $MODULE_ID, chunk $CHUNK_ID)"

echo "==> Файлуудыг $WEBROOT рүү суулгаж байна"
mkdir -p "$WEBROOT/grafana-override"
install -m 644 "$SRC/grafana-gerege.css" "$WEBROOT/grafana-gerege.css"
install -m 644 "$SRC/grafana-gerege.js"  "$WEBROOT/grafana-gerege.js"
install -m 644 "$SRC/logo.webp"          "$WEBROOT/grafana-favicon.webp"

# Landing хуудас. Домэйн солигдвол текст дэх хаягууд ч солигдоно.
sed -e "s/monitor\.nexus\.gerege\.mn/$MONITOR_DOMAIN/g" \
    -e "s/docs\.nexus\.gerege\.mn/$DOCS_HOST/g" \
    -e "s/nexus\.gerege\.mn/$PLATFORM_HOST/g" \
    "$LANDING_SRC" > "$WEBROOT/index.html"

echo "==> Монгол орчуулгын chunk-ийг барьж байна"
python3 - "$SRC/i18n/mn.txt" "$CHUNK_ID" "$MODULE_ID" "$WEBROOT/grafana-override/mn-chunk.js" <<'PY'
import json, sys

src, chunk_id, module_id, out = sys.argv[1:5]

# `key.path = орчуулга` мөрүүдийг Grafana-ийн үүрлэсэн бүтэц рүү задална.
tree, n = {}, 0
for line in open(src, encoding="utf-8"):
    if " = " not in line:
        continue
    key, value = line.rstrip("\n").split(" = ", 1)
    node = tree
    parts = key.strip().split(".")
    for part in parts[:-1]:
        node = node.setdefault(part, {})
    node[parts[-1]] = value
    n += 1

# ASCII-гаар бичнэ: nginx энэ файлыг charset-гүй өгдөг тул кирилл үсэг
# браузарын таамаглалаас хамаарах ёсгүй.
payload = json.dumps(json.dumps(tree, ensure_ascii=True))
open(out, "w", encoding="ascii").write(
    '"use strict";(self.webpackChunkgrafana=self.webpackChunkgrafana||[])'
    f".push([[{chunk_id}],{{{module_id}(e){{e.exports=JSON.parse({payload})}}}}]);"
)
print(f"  {n} түлхүүр орчуулагдсан")
PY

echo "==> Хэлний жагсаалт ба Branding-ийг засаж байна"
docker cp "$GRAFANA_CONTAINER:$BUILD/$LANG_CHUNK" "$WEBROOT/grafana-override/langs.js" >/dev/null
python3 - "$WEBROOT/grafana-override/langs.js" <<'PY'
import sys

path = sys.argv[1]
s = open(path, encoding="utf-8").read()

def esc(text):
    return "".join(c if ord(c) < 128 else "\\u%04x" % ord(c) for c in text)

# Швед хэлний үүрэнд Монгол хэл сууна: Grafana-д `mn` locale байхгүй бөгөөд
# орчуулгууд нь chunk дотор шатсан байдаг тул жагсаалтын нэрийг л сольж,
# орчуулгын chunk-ийг дээр нь орлуулна.
edits = [
    ('name:"Svenska"', 'name:"%s"' % esc("Монгол")),
    ('AppTitle="Grafana"', 'AppTitle="Gerege Nexus"'),
    ('LoginTitle="Welcome to Grafana"', 'LoginTitle="Gerege Nexus \\u00b7 %s"' % esc("Ажиглалт")),
]
for old, new in edits:
    if s.count(old) != 1:
        raise SystemExit(f"АЛДАА: '{old}' {s.count(old)} удаа олдлоо — Grafana-ийн бүтэц өөрчлөгдсөн")
    s = s.replace(old, new)

open(path, "w", encoding="utf-8").write(s)
print("  Монгол, Gerege Nexus, нэвтрэх гарчиг")
PY

echo "==> nginx-ийн snippet-ийг бичиж байна ($SNIPPET)"
mkdir -p "$(dirname "$SNIPPET")"
cat > "$SNIPPET" <<NGINX
# ҮҮСГЭСЭН ФАЙЛ — deploy/scripts/setup_monitor_branding.sh бичдэг. Гараар бүү зас.
#
# Grafana-ийн хоёр chunk-ийг орлуулна. Нэрэн дэх hash нь агуулгаас гардаг тул
# Grafana шинэчлэгдэх бүрд өөрчлөгдөнө — тэр үед скриптийг дахин ажиллуулна.
# Таарахгүй болбол Grafana өөрийн файлаа өгнө: швед хэл эргэж ирнэ, өөр юу ч
# эвдрэхгүй.
location = /grafana/public/build/$MN_CHUNK {
    alias $WEBROOT/grafana-override/mn-chunk.js;
}
location = /grafana/public/build/$LANG_CHUNK {
    alias $WEBROOT/grafana-override/langs.js;
}
NGINX

if [ "$RELOAD" = "1" ]; then
  nginx -t >/dev/null 2>&1 || { nginx -t; die "nginx тохиргоо буруу"; }
  systemctl reload nginx
  say "nginx дахин ачааллав"
fi

echo "==> Шалгаж байна (https://$MONITOR_DOMAIN)"
fail=0

# Шалгалтууд нь браузарын замаар явна: сервер дээрх файл байгаа эсэх биш,
# домэйнээр дамжуулан ЮУ ирж байгаа нь чухал. nginx-ийн location таарахгүй
# бол файл нь байрандаа байсан ч энэ нь унана — тэр яг л хэрэгтэй зан.
check() { # тайлбар, url, олдох ёстой мөр (хоосон бол зөвхөн 200)
  local what="$1" url="$2" want="${3:-}" body
  if ! body="$(curl -fsS --max-time 20 "$url" 2>/dev/null)"; then
    echo "  ДУТУУ $what — хариу ирсэнгүй ($url)"; fail=1; return
  fi
  # `grep -q`-ийг хоолойгоор бүү тэжээ: тэр эхний таарцаараа гарахад өмнөх
  # тушаал SIGPIPE идэж, `pipefail` түүнийг унасан гэж үзнэ — таарсан ч
  # «олдсонгүй» гэж хэлдэг. Bash-ийн загвар тааруулалт хоолойгүй.
  if [ -n "$want" ]; then
    case "$body" in
      *"$want"*) ;;
      *) echo "  ДУТУУ $what — '$want' олдсонгүй ($url)"; fail=1; return ;;
    esac
  fi
  say "OK   $what"
}

check "landing хуудас"      "https://$MONITOR_DOMAIN/"              "$MONITOR_DOMAIN"
check "хэв маяг холбогдсон" "https://$MONITOR_DOMAIN/grafana/login" "grafana-gerege.css"
check "скрипт холбогдсон"   "https://$MONITOR_DOMAIN/grafana/login" "grafana-gerege.js"
check "нэвтрэх гарчиг"      "https://$MONITOR_DOMAIN/grafana/login" "Ажиглалт — Gerege Nexus"
# Файл дотор JSON нь мөр болж хадгалагддаг тул кирилл үсэг ХОЁР ташуутай
# (\\u041d) харагдана — «Нэв». Энэ нь орчуулга бидний файлаас ирж байгаагийн
# баталгаа: Grafana-ийн жинхэнэ швед chunk-д ийм тэмдэгт байхгүй.
check "Монгол орчуулга"     "https://$MONITOR_DOMAIN/grafana/public/build/$MN_CHUNK"   '\\u041d\\u044d\\u0432'
check "Gerege Nexus нэр"    "https://$MONITOR_DOMAIN/grafana/public/build/$LANG_CHUNK" 'AppTitle="Gerege Nexus"'

icon_type="$(curl -fsS -o /dev/null -w '%{content_type}' --max-time 20 \
  "https://$MONITOR_DOMAIN/grafana/public/build/img/fav32.png" 2>/dev/null || true)"
if [ "$icon_type" = "image/webp" ]; then
  say "OK   tab icon"
else
  echo "  ДУТУУ tab icon — content-type '$icon_type', image/webp байх ёстой"; fail=1
fi

if [ "$fail" = "1" ]; then
  die "зарим шалгалт унасан — дээрх мөрүүдийг үзнэ үү"
fi

cat <<DONE

Бэлэн. Grafana-ийн тохиргоонд дараах хоёр мөр байх ёстой (.env.monitoring):

    GRAFANA_DEFAULT_LANGUAGE=sv-SE      # Монгол орчуулга энэ үүрэнд сууна
    GRAFANA_DEFAULT_THEME=desertbloom   # эсвэл light / dark

Дараа нь: docker compose -f deploy/docker-compose.monitoring.yml up -d grafana
DONE
