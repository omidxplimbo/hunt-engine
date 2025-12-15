#!/bin/bash
# Script to initialize SSL certificate with Let's Encrypt

set -e

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

DOMAIN_NAME=${DOMAIN_NAME:-""}
EMAIL=${SSL_EMAIL:-"admin@${DOMAIN_NAME}"}

if [ -z "$DOMAIN_NAME" ]; then
    echo "❌ Error: DOMAIN_NAME is not set in .env file"
    echo "Please create .env file with DOMAIN_NAME=yourdomain.com"
    exit 1
fi

echo "🔐 Initializing SSL certificate for domain: $DOMAIN_NAME"
echo "📧 Using email: $EMAIL"

# Create necessary directories
mkdir -p certbot/conf certbot/www

# Make sure nginx is running first (for acme challenge)
echo "🔄 Starting nginx (initial config)..."
docker compose up -d nginx

# Wait a bit for nginx to start
sleep 3

# Run certbot to obtain certificate
echo "📝 Requesting SSL certificate from Let's Encrypt..."
echo "⚠️  Make sure your DNS A record points to this server's IP!"
echo "⚠️  This may take a few minutes..."

docker compose run --rm certbot certonly \
    --webroot \
    --webroot-path=/var/www/certbot \
    --email "$EMAIL" \
    --agree-tos \
    --no-eff-email \
    --force-renewal \
    -d "$DOMAIN_NAME"

if [ $? -eq 0 ]; then
    echo "✅ SSL certificate obtained successfully!"
    echo "🔄 Restarting Nginx with SSL configuration..."
    docker-compose restart nginx
    sleep 2
    echo "✅ SSL setup complete! Your site should now be accessible at https://$DOMAIN_NAME"
    echo ""
    echo "🔒 Security features enabled:"
    echo "   - HTTPS only (HTTP redirects to HTTPS)"
    echo "   - Domain-only access (IP blocked)"
    echo "   - Security headers"
    echo "   - Rate limiting"
else
    echo "❌ Failed to obtain SSL certificate"
    echo ""
    echo "Common issues:"
    echo "  1. DNS not pointing to this server - check with: dig $DOMAIN_NAME"
    echo "  2. Port 80 not accessible - check firewall"
    echo "  3. Nginx not running - check with: docker-compose ps nginx"
    exit 1
fi
