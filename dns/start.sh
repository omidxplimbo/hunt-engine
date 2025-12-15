#!/bin/bash
# Start script for BIND9 DNS Server

set -e

echo "🌐 Starting BIND9 DNS Server..."

# Generate zone files (uses environment variables from docker-compose)
/usr/local/bin/generate-zone.sh

# Check configuration
echo "🔍 Checking BIND9 configuration..."
named-checkconf /etc/bind/named.conf

if [ $? -eq 0 ]; then
    echo "✅ Configuration is valid"
else
    echo "❌ Configuration error! Checking logs..."
    exit 1
fi

# Check zone file
if [ -n "$DOMAIN_NAME" ] && [ -f "/etc/bind/zones/db.${DOMAIN_NAME}" ]; then
    echo "🔍 Checking zone file..."
    named-checkzone "${DOMAIN_NAME}" "/etc/bind/zones/db.${DOMAIN_NAME}"
    
    if [ $? -eq 0 ]; then
        echo "✅ Zone file is valid"
    else
        echo "❌ Zone file error!"
        exit 1
    fi
fi

# Ensure log directory exists and has correct permissions
mkdir -p /var/log/named
chown -R bind:bind /var/log/named /var/cache/bind

# Start BIND9
echo "🚀 Starting BIND9 DNS Server..."
echo "📍 Listening on port 53 (UDP and TCP)"
exec named -f -u bind -c /etc/bind/named.conf
