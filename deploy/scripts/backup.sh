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
# Өөр байршил руу илгээх (бүгд хоосон бол алхам нь бүхэлдээ алгасагдана):
#
#   BACKUP_AGE_RECIPIENT — age-ийн НИЙТИЙН түлхүүр. Хостод зөвхөн энэ байна:
#                          эвдэрсэн платформ өөрийн илгээсэн зүйлээ уншиж
#                          чадахгүй. Хувийн түлхүүр нь операторт байна.
#   BACKUP_S3_ENDPOINT / BACKUP_S3_BUCKET / BACKUP_S3_KEY / BACKUP_S3_SECRET
#
# ЭНЭ СКРИПТ НЬ ХАНГАЛТТАЙ ГЭДЭГ АМЛАЛТ БИШ. Нэг хостын дискэн дээрх нөөцлөлт
# нь тэр хостыг алдвал хамт алга болно: docs/OPERATIONS.md бичсэнээр
# өөр байршил руу хуулах (rclone, rsync, S3) нь дараагийн алхам. Гэхдээ
# байхгүйгээс энэ дээр нь бүтээх боломжтой зүйл байсан нь дээр.

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/var/backups/gerege-nexus}"
BACKUP_KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-gerege_nexus_postgres}"
POSTGRES_DB="${POSTGRES_DB:-platform_db}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
TEXTFILE_DIR="${TEXTFILE_DIR:-/var/lib/node_exporter}"
BACKUP_AGE_RECIPIENT="${BACKUP_AGE_RECIPIENT:-}"
BACKUP_S3_ENDPOINT="${BACKUP_S3_ENDPOINT:-}"
BACKUP_S3_BUCKET="${BACKUP_S3_BUCKET:-}"
BACKUP_S3_KEY="${BACKUP_S3_KEY:-}"
BACKUP_S3_SECRET="${BACKUP_S3_SECRET:-}"
offsite_ok=0

# Хэмжигдэхгүй нөөцлөлт нь нөөцлөлт байхгүйтэй бараг адил: cron-ий чимээгүй
# бүтэлгүйтэл нь сэргээх өдрөө л илэрдэг. Өгөгдлийн сан дахь мөр нь консол
# уншдаг, харин энэ нь Prometheus уншдаг — өөрөөр хэлбэл шөнө дунд хэн нэгэнд
# сэрэмжлүүлэг илгээж чадах цорын ганц хувилбар. Бичих арга нь TLS-ийн
# хугацааны ажилтай яг ижил (docs/OPERATIONS.md): атомик бичилт, учир нь
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
        # Тохируулаагүй суулгац дээр 0 хэвээр байна, тэр нь зөв: хуулбар өөр
        # газар байхгүй гэдэг нь хэмжигдэх ёстой баримт.
        echo "# HELP nexus_backup_offsite_ok Whether the encrypted copy reached the off-site store"
        echo "# TYPE nexus_backup_offsite_ok gauge"
        echo "nexus_backup_offsite_ok ${offsite_ok}"
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

# Өөр байршил руу.
#
# Дискэн дээрх нөөцлөлт нь тэр дискийг алдвал хамт алга болно. Энэ алхам нь
# хуулбарыг өөр газар үлдээнэ — гэхдээ эхлээд ШИФРЛЭНЭ. Хостод зөвхөн нийтийн
# түлхүүр байгаа тул эвдэрсэн платформ ч, нөөцийн санг барьсан хэн ч уншиж
# чадахгүй.
#
# Аль нэг тохиргоо дутуу бол алхам бүхэлдээ алгасагдана: тохируулаагүй суулгац
# энэ скриптийг ажиллуулж чадах ёстой.
offsite() {
    [ -n "${BACKUP_AGE_RECIPIENT}" ] || return 0
    [ -n "${BACKUP_S3_ENDPOINT}" ] && [ -n "${BACKUP_S3_BUCKET}" ] || return 0
    [ -n "${BACKUP_S3_KEY}" ] && [ -n "${BACKUP_S3_SECRET}" ] || return 0
    command -v age >/dev/null 2>&1 || { echo "backup: age суугаагүй" >&2; return 1; }

    local enc="${target}.age" key host date_hdr sig
    if ! age -r "${BACKUP_AGE_RECIPIENT}" -o "${enc}" "${target}"; then
        rm -f "${enc}"
        echo "backup: шифрлэж чадсангүй" >&2
        return 1
    fi

    key="$(basename "${enc}")"
    # AWS SigV4-ийн оронд S3-ийн хуучин, гэхдээ MinIO дэмждэг presigned биш
    # энгийн гарын үсэг ашиглахгүй: mc-г контейнерээс дуудна. Хостод шинэ
    # хоёртын файл суулгахгүй, MinIO-гийн өөрийнх нь клиент аль хэдийн бий.
    if ! docker run --rm --network host \
            -e MC_HOST_store="https://${BACKUP_S3_KEY}:${BACKUP_S3_SECRET}@${BACKUP_S3_ENDPOINT#https://}" \
            -v "${enc}:/upload/${key}:ro" \
            minio/mc:latest cp --quiet "/upload/${key}" "store/${BACKUP_S3_BUCKET}/${key}" >/dev/null 2>&1; then
        rm -f "${enc}"
        echo "backup: өөр байршил руу илгээж чадсангүй" >&2
        return 1
    fi
    rm -f "${enc}"
    offsite_ok=1
    echo "backup: өөр байршилд ${BACKUP_S3_BUCKET}/${key}"
    return 0
}

offsite || true

record true "${size}" "${target}"
write_metrics true "${size}"
echo "backup: ${target} (${size} байт)"
