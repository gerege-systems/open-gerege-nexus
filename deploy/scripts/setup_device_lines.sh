#!/bin/bash
# Төхөөрөмжийн domain шугамуудыг асаах.
#
# Шугамууд backend-ээсээ хэзээ ч салахгүй: ижил host, ижил nginx, ижил
# upstream (3008 frontend, 8082 API). Шугам гэдэг нь өөр сервис биш, зөвхөн
# өөр server_name — тиймээс энэ скрипт нэг vhost суулгаад л дуусна.
#
# nginx хийж ЧАДАХГҮЙ ганц зүйл бол нэрийг resolve болгох. Тэр нь DNS-ийн
# ажил: шугам бүрт `A → <энэ серверийн IP>` бичлэг хэрэгтэй, яг одоо
# ds.nexus.gerege.mn дээр байгаа шиг. Тиймээс скрипт хамгийн түрүүнд DNS-ийг
# шалгаад, resolve болоогүй нэр байвал зогсоно — эсрэг дараалал нь клиентийг
# байхгүй host руу чиглүүлж, "A server with the specified hostname could not
# be found" гэсэн алдаа өгдөг.

set -euo pipefail

# Шугам бүр нэг form factor. Платформ биш: ширээн дээрх Mac ба Windows хоёр
# `desktop`-ыг хуваалцана.
LINES=(
  desktop.nexus.gerege.mn
  mobile.nexus.gerege.mn
  kiosk.nexus.gerege.mn
  pos.nexus.gerege.mn
)
EMAIL="admin@gerege.mn"
VHOST_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/nginx/device-lines.nexus.gerege.mn.conf"
VHOST_DST="/etc/nginx/sites-available/device-lines.nexus.gerege.mn.conf"

echo "==== Device domain lines ===="

# 1. DNS. Энэ серверийн олон нийтийн IP-тэй тааруулж шалгана — өөр газар руу
#    заасан бичлэг нь resolve болж байгаа хэрнээ энэ nginx рүү ирэхгүй.
SERVER_IP="$(curl -fsS https://api.ipify.org || true)"
echo "Энэ серверийн IP: ${SERVER_IP:-тодорхойлж чадсангүй}"

missing=()
for host in "${LINES[@]}"; do
    resolved="$(dig +short "$host" A | tail -1)"
    if [[ -z "$resolved" ]]; then
        printf '  %-26s ✗ DNS бичлэг алга\n' "$host"
        missing+=("$host")
    elif [[ -n "$SERVER_IP" && "$resolved" != "$SERVER_IP" ]]; then
        printf '  %-26s ✗ %s руу заасан байна (энэ сервер биш)\n' "$host" "$resolved"
        missing+=("$host")
    else
        printf '  %-26s ✓ %s\n' "$host" "$resolved"
    fi
done

if (( ${#missing[@]} )); then
    echo
    echo "DNS-д доорх бичлэгүүдийг нэмнэ үү (ds.nexus.gerege.mn-тэй яг ижил хэлбэр):"
    echo
    for host in "${missing[@]}"; do
        printf '  %-26s  A  %s\n' "${host%%.nexus.gerege.mn}" "${SERVER_IP:-<энэ серверийн IP>}"
    done
    cat <<EOF

Бүх дэд домэйныг нэг дор шийдэх бол `*.nexus.gerege.mn A ${SERVER_IP}` гэсэн
wildcard бичлэг мөн болно. Гэхдээ энэ vhost нь server_name-ээ нэрлэн бичдэг
тул wildcard нь ирээдүйн өөр дэд домэйныг санамсаргүй залгихгүй — nginx
яг тохирох server_name-ыг үргэлж эхэлж сонгоно (ds.nexus.gerege.mn аюулгүй).

Бичлэгүүд тархсаны дараа энэ скриптийг дахин ажиллуулна уу.
EOF
    exit 1
fi

# 2. vhost. Upstream нь nexus.gerege.mn-тэй ИЖИЛ — backend цор ганц хэвээр.
echo
echo "vhost суулгаж байна: ${VHOST_DST}"
sudo cp "$VHOST_SRC" "$VHOST_DST"
sudo ln -sf "$VHOST_DST" /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx || sudo service nginx reload

# 3. TLS. DNS resolve болсон тул HTTP-01 ажиллана — wildcard гэрчилгээ ба
#    түүний DNS-01 challenge хэрэггүй.
if ! command -v certbot &> /dev/null; then
    sudo apt-get update && sudo apt-get install -y certbot python3-certbot-nginx
fi
echo
echo "Гэрчилгээ авч байна (${#LINES[@]} нэр, нэг гэрчилгээ)…"
certbot_args=()
for host in "${LINES[@]}"; do certbot_args+=(-d "$host"); done
sudo certbot --nginx \
    "${certbot_args[@]}" \
    --non-interactive --agree-tos --email "$EMAIL" --redirect --reinstall

# 4. API-гийн origin allowlist. Үүнгүйгээр cookie-гаар баталгаажсан бичих
#    үйлдэл бүр "origin not allowed" гэж 403 өгнө.
cat <<EOF

==== Үлдсэн хоёр алхам ====

1. /opt/open-gerege-nexus/.env дотор:

   DEVICE_LINE_ORIGINS=$(printf 'https://%s,' "${LINES[@]}" | sed 's/,$//')

   дараа нь:  docker compose -f docker-compose.prod.yml up -d api

2. Клиентүүдийг шугам руу нь чиглүүлнэ — ЗӨВХӨН одоо, өмнө нь биш.
   native-apps/shared/device_lines.json-ы \$provisioning заасан мөрүүд:
     macOS    native-apps/desktop/macos/NativeSettings.swift        → activeOrigin
     Windows  native-apps/desktop/windows/ShellProfile.cs           → ActiveOrigin
     iOS      .../GeregeShellKit/DeviceLine.swift           → origin
     Android  .../mn/gerege/nexus/DeviceLine.kt             → origin
   мөн device_lines.json дотор provisioned: true болгоно.

==== Шалгах ====
EOF
for host in "${LINES[@]}"; do
    printf '  %-26s ' "$host"
    curl -fsS -o /dev/null -w 'HTTP %{http_code}\n' "https://${host}/apps" || echo "хүрсэнгүй"
done
