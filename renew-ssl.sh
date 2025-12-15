#!/bin/bash
# Script to renew SSL certificate (should be run periodically, e.g., via cron)

set -e

echo "🔄 Renewing SSL certificate..."

docker compose run --rm certbot renew

# Reload nginx to use new certificates
docker compose exec nginx nginx -s reload

echo "✅ SSL certificate renewal complete!"
