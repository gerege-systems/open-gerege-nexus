#!/bin/sh
# Угсраад байршуулна. Ганц аргумент нь ssh-ийн alias.
#
#   sh deploy.sh nexus-root
#
# Угсралт нь алсын хост дээр явагдана: тэнд Docker байгаа, мөн macOS дээрх
# Docker Desktop-ийн хавтас хуваалцах зан төлөвөөс хамаарахгүй.
set -e
HOST="${1:?usage: deploy.sh <ssh-host>}"
cd "$(dirname "$0")"

node stage.mjs
tar czf /tmp/nexus-docs-build.tgz build
scp -q /tmp/nexus-docs-build.tgz "$HOST:/root/docs-build.tgz"
rm -f /tmp/nexus-docs-build.tgz

ssh "$HOST" 'set -e
  rm -rf /root/docs-build && mkdir -p /root/docs-build
  cd /root/docs-build && tar xzf /root/docs-build.tgz
  # Гаралтыг файлд бичээд ДАРАА нь шүүнэ. Шууд grep рүү дамжуулбал pipe нь
  # угсралтын гарах кодыг залгидаг тул strict дээр унасан build ч гэсэн
  # нийтлэгдэж, хуучин сайт хагас шинэчлэгдэнэ.
  docker run --rm -v /root/docs-build/build:/w -w /w python:3.12-slim sh -c "
    pip install --quiet --no-cache-dir mkdocs==1.6.1 mkdocs-material==9.7.7 mkdocs-static-i18n==1.3.0 pymdown-extensions >/dev/null 2>&1 &&
    mkdocs build --strict
  " > /root/docs-build/build.log 2>&1 || {
    grep -E "WARNING|ERROR|Aborted" /root/docs-build/build.log | head -20
    echo "build failed — nothing published" >&2
    exit 1
  }
  grep -E "Documentation built" /root/docs-build/build.log
  rm -rf /var/www/docs && mkdir -p /var/www/docs
  cp -r /root/docs-build/build/site/. /var/www/docs/
  echo "published to /var/www/docs"'
