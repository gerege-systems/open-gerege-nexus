#!/usr/bin/env bash
#
# Хэдэн хувийн аппын ажил цөмийг хөдөлгөж байна вэ.
#
#   scripts/core-boundary-metric.sh              # Үе 0-ээс хойш
#   scripts/core-boundary-metric.sh -300         # түүхэн жишиг
#   scripts/core-boundary-metric.sh <sha>..HEAD  # дурын цонх
#
# docs/CORE_BOUNDARY_PLAN.md §9-ийн хэмжүүр. Тэнд инлайнаар бичигдсэн
# хувилбар нь `git log -300` буюу бүтэн түүхийг уншдаг байсан бөгөөд тэр нь
# заадсын ажлын дараа буруу хариу өгдөг: хэмжигдэж буй 300 коммит бүгд
# өөрчлөлтөөс өмнөх тул тоо хөдлөхгүй. Анхдагч цонх нь Үе 0 merge хийгдсэн
# цэгээс эхэлдэг болов.
#
# Хоёр дүрэм инлайн хувилбараас өөр:
#
#   1. Merge коммит тоологдохгүй. Merge нь хоёр талынхаа бүх файлыг
#      харуулдаг тул хүрсэн гэж бүртгэгдэнэ.
#   2. Аппын коммит гэдэг нь аппын ӨӨРИЙН хавтас хөндсөн коммит
#      (`internal/apps/<app>/`). `internal/apps/runtime.go` дангаараа
#      өөрчлөгдөх нь платформын ажил, аппынх биш.
#
# Хэмжүүр нь ТҮҮХИЙГ уншдаг гэдгийг санах хэрэгтэй: аппын ажил хуримтлагдах
# хүртэл тоо утга учиртай болохгүй. Түүхэн хурд нь өдөрт ~8 аппын коммит
# байсан тул 30 коммит цуглахад 3-4 хоног. Түүнээс цөөн коммиттой цонхны
# хариуг шийдвэрт ашиглаж болохгүй — docs/adr/0004-a-pilot-that-did-not-ship.md.

set -euo pipefail

# Үе 0 merge хийгдсэн цэг: цөмийн заадсын ажил эхэлсэн газар.
PHASE_0=7423245

RANGE="${1:-$PHASE_0..HEAD}"

# Цөмийн заадас гэж юуг хэлэх вэ — §2.1-ийн хэмжсэн жагсаалт.
SEAM='^(backend/internal/operator/|backend/pkg/nexus/|frontend/lib/api.ts|frontend/lib/i18n/|frontend/components/Layout.tsx|catalog/|backend/db/migrations/)'

total=0
touched=0

for commit in $(git log --no-merges --format=%H "$RANGE" -- backend/internal/apps); do
    changed=$(git show --name-only --pretty=format: "$commit")

    # Аппын өөрийн хавтсанд хүрээгүй бол аппын ажил биш.
    echo "$changed" | grep -qE '^backend/internal/apps/[a-z_]+/' || continue
    total=$((total + 1))

    if echo "$changed" | grep -qE "$SEAM"; then
        touched=$((touched + 1))
        printf '  цөмд хүрсэн  %s\n' "$(git show -s --format='%h %s' "$commit" | cut -c1-72)"
    fi
done

echo
if [ "$total" -eq 0 ]; then
    echo "Цонхонд ($RANGE) аппын коммит алга. Хэмжих зүйл байхгүй."
    exit 0
fi

printf 'Цонх : %s\n' "$RANGE"
printf 'Дүн  : %d / %d = %s%%\n' "$touched" "$total" \
    "$(echo "scale=1; 100*$touched/$total" | bc)"

if [ "$total" -lt 30 ]; then
    echo
    echo "АНХААР: аппын коммит $total-хан. Түүхэн хурдаар 30 коммит цуглахад 3-4"
    echo "хоног. Үүнээс цөөн дээжийн хариуг шийдвэрт ашиглаж болохгүй."
fi
