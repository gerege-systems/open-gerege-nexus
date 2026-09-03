#!/usr/bin/env bash
# Write each certbot certificate's expiry into node_exporter's textfile
# collector, so Prometheus can alert before a certificate lapses.
#
# This lives here rather than in a document because a recipe that has to be
# pasted onto every new host is a recipe every new host goes without: this one
# was missing on nexus.gerege.mn for months, and a missing measurement looks
# exactly like a healthy one (NexusTLSExpiryUnknown says so, and is easy to
# read as "a certificate problem").
#
# Install:
#   install -m 755 deploy/scripts/nexus-tls-expiry.sh /usr/local/bin/
#   install -d /var/lib/node_exporter
#   echo '17 * * * * root /usr/local/bin/nexus-tls-expiry.sh' > /etc/cron.d/nexus-tls-expiry
#
# node_exporter is already started with
# --collector.textfile.directory=/host/var/lib/node_exporter, so nothing else
# is needed on the monitoring side.
set -euo pipefail

OUT_DIR="${TEXTFILE_DIR:-/var/lib/node_exporter}"
OUT="$OUT_DIR/nexus_tls.prom"
LIVE=/etc/letsencrypt/live

install -d "$OUT_DIR"

# Written to a temporary file and moved into place: node_exporter reads this
# directory on its own schedule and a half-written file is a parse error, which
# drops *every* metric in it rather than just the missing line.
TMP="$(mktemp "$OUT.XXXXXX")"
trap 'rm -f "$TMP"' EXIT

{
  echo "# HELP nexus_tls_not_after_timestamp_seconds Certificate expiry, seconds since epoch."
  echo "# TYPE nexus_tls_not_after_timestamp_seconds gauge"
  for cert in "$LIVE"/*/cert.pem; do
    [ -e "$cert" ] || continue
    domain="$(basename "$(dirname "$cert")")"
    not_after="$(openssl x509 -enddate -noout -in "$cert" | cut -d= -f2)"
    epoch="$(date -u -d "$not_after" +%s)"
    printf 'nexus_tls_not_after_timestamp_seconds{domain="%s"} %s\n' "$domain" "$epoch"
  done
} > "$TMP"

chmod 644 "$TMP"
mv "$TMP" "$OUT"
trap - EXIT
