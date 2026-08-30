#!/usr/bin/env bash
#
# Gerege Nexus — өгөгдлийн сангийн нөөцлөлт.
#
# Энэ платформ дээр CP-4 хүртэл нөөцлөлт БАЙГААГҮЙ. Тиймээс энэ файл нь
# "консол дээр харуулах статус" гэхээсээ илүү, эхлээд нөөцлөлт өөрөө юм.
#
# Хийдэг зүйл нь гурав: pg_dump авах, хуучныг цэвэрлэх, үр дүнг өгөгдлийн санд
# бүртгэх. Гурав дахь нь консол уншдаг мөр (`platform_backups`) бөгөөд түүнгүй
# бол нөөцлөлт ажиллаж байгаа эсэхийг хэн ч мэдэхгүй — cron-ий чимээгүй
# бүтэлгүйтэл нь сэргээх өдрөө л илэрдэг.
#
# Cron дээр (жишээ нь өдөр бүр 03:15 цагт):
#
#   15 3 * * * /opt/gerege-nexus/deploy/scripts/backup.sh >> /var/log/nexus-backup.log 2>&1
#
# Тохируулга (env эсвэл дуудахын өмнө export):
#
#   BACKUP_DIR       — хаана хадгалах (анхдагч /var/backups/gerege-nexus)
#   BACKUP_KEEP_DAYS — хэдэн хоног хадгалах (анхдагч 14)
#   POSTGRES_CONTAINER — postgres контейнерийн нэр (анхдагч gerege_nexus_postgres)
#   POSTGRES_DB / POSTGRES_USER — анхдагч platform_db / postgres
#   TEXTFILE_DIR     — node_exporter-ийн textfile хавтас
#                      (анхдагч /var/lib/node_exporter, хоосон бол бичихгүй)
#
# ЭНЭ СКРИПТ НЬ ХАНГАЛТТАЙ ГЭДЭГ АМЛАЛТ БИШ. Нэг хостын дискэн дээрх нөөцлөлт
# нь тэр хостыг алдвал хамт алга болно: docs/CONTROL_PLANE.md §4и-д бичсэнээр
# өөр байршил руу хуулах (rclone, rsync, S3) нь дараагийн алхам. Гэхдээ
# байхгүйгээс энэ дээр нь бүтээх боломжтой зүйл байсан нь дээр.

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/var/backups/gerege-nexus}"
BACKUP_KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-gerege_nexus_postgres}"
POSTGRES_DB="${POSTGRES_DB:-platform_db}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
TEXTFILE_DIR="${TEXTFILE_DIR:-/var/lib/node_exporter}"

# Хэмжигдэхгүй нөөцлөлт нь нөөцлөлт байхгүйтэй бараг адил: cron-ий чимээгүй
# бүтэлгүйтэл нь сэргээх өдрөө л илэрдэг. Өгөгдлийн сан дахь мөр нь консол
# уншдаг, харин энэ нь Prometheus уншдаг — өөрөөр хэлбэл шөнө дунд хэн нэгэнд
# сэрэмжлүүлэг илгээж чадах цорын ганц хувилбар. Бичих арга нь TLS-ийн
# хугацааны ажилтай яг ижил (docs/MONITORING.md §8): атомик бичилт, учир нь
# node_exporter хагас бичигдсэн файлыг уншиж болохгүй.
write_metrics() {
    local ok="$1" size="$2"
    [ -n "${TEXTFILE_DIR}" ] && [ -d "${TEXTFILE_DIR}" ] || return 0
    local out="${TEXTFILE_DIR}/nexus_backup.prom" tmp
    tmp="$(mktemp "${out}.XXXXXX")" || return 0
    {
        echo "# HELP nexus_backup_last_run_timestamp_seconds When the backup job last ran, successful or not"
        echo "# TYPE nexus_backup_last_run_timestamp_seconds gauge"
        echo "nexus_backup_last_run_timestamp_seconds $(date +%s)"
        echo "# HELP nexus_backup_last_success_timestamp_seconds When a backup last succeeded"
        echo "# TYPE nexus_backup_last_success_timestamp_seconds gauge"
        if [ "${ok}" = "true" ]; then
            echo "nexus_backup_last_success_timestamp_seconds $(date +%s)"
            echo "# HELP nexus_backup_last_size_bytes Size of the last successful dump"
            echo "# TYPE nexus_backup_last_size_bytes gauge"
            echo "nexus_backup_last_size_bytes ${size}"
        else
            # Өмнөх амжилтын мөчийг хадгална — эс бөгөөс нэг бүтэлгүйтэл нь
            # "хэзээ ч амжилттай болоогүй"-тэй ялгагдахаа болино.
            local previous
            previous="$(awk '/^nexus_backup_last_success_timestamp_seconds /{print $2}' "${out}" 2>/dev/null)"
            [ -n "${previous}" ] && echo "nexus_backup_last_success_timestamp_seconds ${previous}"
        fi
        echo "# HELP nexus_backup_last_ok Whether the last run succeeded"
        echo "# TYPE nexus_backup_last_ok gauge"
        echo "nexus_backup_last_ok $([ "${ok}" = "true" ] && echo 1 || echo 0)"
    } > "${tmp}"
    chmod 0644 "${tmp}"
    mv -f "${tmp}" "${out}"
}

stamp="$(date +%Y%m%d-%H%M%S)"
target="${BACKUP_DIR}/nexus-${stamp}.sql.gz"
started="$(date --iso-8601=seconds 2>/dev/null || date +%Y-%m-%dT%H:%M:%S%z)"

mkdir -p "${BACKUP_DIR}"

# Бүртгэлийг үргэлж бичнэ — амжилттай ч, амжилтгүй ч. Амжилтгүй нөөцлөлтийн
# тухай чимээгүй байх нь нөөцлөлт огт хийхгүй байхтай ижил хор уршигтай.
record() {
    local ok="$1" size="$2" detail="$3"
    docker exec -i "${POSTGRES_CONTAINER}" \
        psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" \
        -c "INSERT INTO platform_backups (kind, started_at, finished_at, size_bytes, ok, detail)
            VALUES ('backup', '${started}', NOW(), ${size}, ${ok}, \$detail\$${detail}\$detail\$)" \
        >/dev/null 2>&1 || echo "backup: өгөгдлийн санд бүртгэж чадсангүй" >&2
}

if ! docker exec -i "${POSTGRES_CONTAINER}" \
        pg_dump -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" --no-owner --clean --if-exists \
        2>/tmp/nexus-backup.err | gzip -9 > "${target}"; then
    detail="$(tail -c 500 /tmp/nexus-backup.err || true)"
    rm -f "${target}"
    record false NULL "pg_dump failed: ${detail}"
    write_metrics false 0
    echo "backup: pg_dump амжилтгүй" >&2
    exit 1
fi

size="$(wc -c < "${target}" | tr -d ' ')"

# Хоосон гаралт нь амжилттай харагдах бүтэлгүйтлийн сонгодог хэлбэр: pg_dump
# алдаагүй дуусаад юу ч бичээгүй байх. Хэдэн килобайтаас бага бол сэжигтэй.
if [ "${size}" -lt 10240 ]; then
    record false "${size}" "the dump is only ${size} bytes"
    write_metrics false "${size}"
    echo "backup: гаралт хэтэрхий жижиг (${size} байт)" >&2
    exit 1
fi

find "${BACKUP_DIR}" -name 'nexus-*.sql.gz' -mtime "+${BACKUP_KEEP_DAYS}" -delete || true

record true "${size}" "${target}"
write_metrics true "${size}"
echo "backup: ${target} (${size} байт)"
