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
  docker run --rm -v /root/docs-build/build:/w -w /w python:3.12-slim sh -c "
    pip install --quiet --no-cache-dir mkdocs==1.6.1 mkdocs-material==9.7.7 pymdown-extensions >/dev/null 2>&1 &&
    mkdocs build --strict
  " 2>&1 | grep -E "WARNING|ERROR|Documentation built|Aborted"
  rm -rf /var/www/docs && mkdir -p /var/www/docs
  cp -r /root/docs-build/build/site/. /var/www/docs/
  echo "published to /var/www/docs"'
