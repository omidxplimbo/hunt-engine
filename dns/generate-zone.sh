#!/bin/bash
# Script to generate DNS zone file for the domain

set -e

# Load environment variables
if [ -f ../.env ]; then
    export $(cat ../.env | grep -v '^#' | xargs)
fi

DOMAIN_NAME=${DOMAIN_NAME:-""}
SERVER_IP=${SERVER_IP:-""}

if [ -z "$DOMAIN_NAME" ]; then
    echo "❌ Error: DOMAIN_NAME is not set in .env file"
    exit 1
fi

if [ -z "$SERVER_IP" ]; then
    echo "⚠️  Warning: SERVER_IP is not set, using default detection"
    # Try to get public IP
    SERVER_IP=$(curl -s ifconfig.me || curl -s ipinfo.io/ip || echo "127.0.0.1")
fi

echo "🌐 Generating DNS zone for: $DOMAIN_NAME"
echo "📍 Server IP: $SERVER_IP"

# Generate zone file
cat > "/etc/bind/zones/db.${DOMAIN_NAME}" <<EOF
; DNS Zone File for ${DOMAIN_NAME}
; Generated automatically

\$TTL    3600
\$ORIGIN ${DOMAIN_NAME}.

@       IN      SOA     ns1.${DOMAIN_NAME}. admin.${DOMAIN_NAME}. (
                        $(date +%s)    ; Serial
                        3600            ; Refresh
                        1800            ; Retry
                        604800          ; Expire
                        86400 )         ; Minimum TTL

; Name Servers
@               IN      NS      ns1.${DOMAIN_NAME}.
@               IN      NS      ns2.${DOMAIN_NAME}.

; Name Server Records (point to this server)
ns1             IN      A       ${SERVER_IP}
ns2             IN      A       ${SERVER_IP}

; Main Domain (A Record)
@               IN      A       ${SERVER_IP}
*               IN      A       ${SERVER_IP}

; Common subdomains (optional - uncomment if needed)
; www            IN      A       ${SERVER_IP}
; api            IN      A       ${SERVER_IP}

; Mail Records (optional - uncomment if you have mail server)
; @               IN      MX      10      mail.${DOMAIN_NAME}.
; mail            IN      A       ${SERVER_IP}

EOF

# Generate named.conf.zones (will be included in named.conf.local)
cat > "/etc/bind/named.conf.zones" <<EOF
// Zone configuration for ${DOMAIN_NAME}
// Generated automatically

zone "${DOMAIN_NAME}" {
    type master;
    file "/etc/bind/zones/db.${DOMAIN_NAME}";
    allow-update { none; };
    allow-query { any; };
};

// Reverse zone for localhost (required by BIND9)
zone "0.0.127.in-addr.arpa" {
    type master;
    file "/etc/bind/zones/db.127.0.0";
    allow-update { none; };
};

EOF

# Generate reverse zone for localhost (required by BIND9)
cat > "/etc/bind/zones/db.127.0.0" <<EOF
; Reverse zone for 127.0.0.1
\$TTL    3600
@       IN      SOA     localhost. root.localhost. (
                        1
                        3600
                        1800
                        604800
                        86400 )
        IN      NS      localhost.
1       IN      PTR     localhost.
EOF

echo "✅ Zone file generated successfully!"
echo "📝 Zone file location: /etc/bind/zones/db.${DOMAIN_NAME}"
echo "🌐 Name Servers: ns1.${DOMAIN_NAME} and ns2.${DOMAIN_NAME}"
echo "📋 Set these Name Servers in your domain registrar:"
echo "   ns1.${DOMAIN_NAME}"
echo "   ns2.${DOMAIN_NAME}"
