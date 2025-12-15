FROM ubuntu:22.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install BIND9 and required tools
RUN apt-get update && \
    apt-get install -y bind9 bind9utils dnsutils curl iputils-ping && \
    rm -rf /var/lib/apt/lists/*

# Create necessary directories
RUN mkdir -p /var/cache/bind /var/log/named /etc/bind/zones

# Copy configuration files
COPY named.conf.options /etc/bind/named.conf.options
COPY named.conf.local /etc/bind/named.conf.local

# Create main named.conf (includes all config files)
RUN echo 'include "/etc/bind/named.conf.options";' > /etc/bind/named.conf && \
    echo 'include "/etc/bind/named.conf.local";' >> /etc/bind/named.conf

COPY generate-zone.sh /usr/local/bin/generate-zone.sh
RUN chmod +x /usr/local/bin/generate-zone.sh

# Set permissions
RUN chown -R bind:bind /var/cache/bind /var/log/named /etc/bind/zones

# Expose DNS port
EXPOSE 53/udp 53/tcp 953/tcp

# Start script
COPY start.sh /start.sh
RUN chmod +x /start.sh

CMD ["/start.sh"]