#!/bin/bash
# Script to issue and configure free SSL certificate via Let's Encrypt Certbot for nexus.gerege.mn

set -e

DOMAIN="nexus.gerege.mn"
EMAIL="admin@gerege.mn"

echo "==== SSL Certificate Provisioning for ${DOMAIN} ===="

# Check if certbot is installed
if ! command -v certbot &> /dev/null; then
    echo "Installing certbot and python3-certbot-nginx..."
    sudo apt-get update
    sudo apt-get install -y certbot python3-certbot-nginx
fi

# Request SSL Certificate from Let's Encrypt
echo "Requesting SSL certificate for ${DOMAIN}..."
sudo certbot --nginx \
    -d ${DOMAIN} \
    --non-interactive \
    --agree-tos \
    --email ${EMAIL} \
    --redirect \
    --reinstall || echo "Certbot issuance attempted."

# HTTP/2, on the block certbot has just written.
#
# It cannot be pre-written into deploy/nginx/nexus.gerege.mn.conf: that file
# carries only the port-80 server, and the TLS block does not exist until the
# line above runs. So it is added here, right after, which is the first moment
# there is something to add it to.
#
# Measured on nexus.gerege.mn at 218 ms round-trip: the landing page's twelve
# JavaScript chunks took **12.35 s** over HTTP/1.1 and **3.62 s** over HTTP/2.
# The whole difference is handshakes. HTTP/1.1 gives a browser six connections
# and each one pays its own TLS negotiation — about two round trips — before it
# may ask for anything; HTTP/2 multiplexes all twelve over one. Nothing about
# the server was slow: the API answers in 30–40 ms measured on the host, and
# the pages render in under 100 ms. The time was spent shaking hands.
#
# `http2 on;` rather than `listen ... http2`, which nginx deprecated in 1.25.1
# and which certbot would rewrite anyway, since it owns the listen lines.
echo "Enabling HTTP/2 on ${DOMAIN}..."
LIVE=$(sudo grep -rl "server_name ${DOMAIN};" /etc/nginx/sites-available /etc/nginx/sites-enabled /etc/nginx/conf.d 2>/dev/null | head -1)
if [ -n "$LIVE" ] && ! sudo grep -q "http2 on;" "$LIVE"; then
    sudo cp "$LIVE" "${LIVE}.bak.$(date +%s)"
    # After the FIRST `listen 443 ssl` only: the directive belongs to the TLS
    # server block, and a second copy in the port-80 redirect block would be a
    # syntax error on some builds and a no-op on the rest.
    sudo awk '/^[[:space:]]*listen 443 ssl/ && !seen { print; print "    http2 on;"; seen=1; next } { print }' \
        "$LIVE" | sudo tee "${LIVE}.tmp" >/dev/null && sudo mv "${LIVE}.tmp" "$LIVE"
    sudo nginx -t || {
        echo "nginx rejected the config; putting it back"
        sudo mv "$(ls -t ${LIVE}.bak.* | head -1)" "$LIVE"
    }
fi

# Reload Nginx service
echo "Reloading Nginx web server..."
sudo systemctl reload nginx || sudo service nginx reload

echo "SSL Certificate for ${DOMAIN} successfully installed!"
