# ================================================
# Stage 1: Golang Builder (کامپایل ابزارهای Go و برنامه اصلی)
# ================================================
FROM golang:1.25.7-alpine AS go_builder
RUN apk add --no-cache git build-base
WORKDIR /app

# --- نصب ابزارهای شکار (Recon Tools) ---

# فاز ۱: کشف
ENV GOPROXY=https://goproxy.io,direct
RUN apk add --no-cache git
RUN go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
RUN go install -v github.com/tomnomnom/assetfinder@latest
RUN go install -v github.com/projectdiscovery/alterx/cmd/alterx@latest
RUN go install -v github.com/projectdiscovery/dnsx/cmd/dnsx@latest
RUN go install -v github.com/glebarez/cero@latest
RUN go install -v github.com/d3mondev/puredns/v2@latest

# puredns نیاز به massdns دارد (برای resolve سریع)
RUN git clone --depth 1 https://github.com/blechschmidt/massdns.git /tmp/massdns \
    && make -C /tmp/massdns \
    && mkdir -p /out \
    && cp /tmp/massdns/bin/massdns /out/massdns

# فاز ۲: پروبینگ
RUN go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest
RUN go install -v github.com/projectdiscovery/cdncheck/cmd/cdncheck@latest
RUN go install -v github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest

# فاز ۳: خزش و کشف محتوا
RUN go install github.com/tomnomnom/waybackurls@latest
RUN go install github.com/lc/gau/v2/cmd/gau@latest
RUN go install github.com/projectdiscovery/katana/cmd/katana@latest

# --- بیلد برنامه اصلی ---
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hunt-engine-api ./cmd/server

# ================================================
# Stage 2: Final Runner
# ================================================
FROM alpine:3.18
LABEL maintainer="Omid Security Researcher"

# 👇 نصب پایتون و ابزارهای بیلد مورد نیاز برای Waymore (gcc, headers, etc)
# اضافه شدن: gcc python3-dev musl-dev linux-headers libffi-dev + nmap + bash
RUN apk --no-cache add ca-certificates bind-tools jq curl python3 py3-pip git gcc python3-dev musl-dev linux-headers libffi-dev nmap bash unzip

# نصب Waymore
# نکته: Waymore نیاز به پایتون دارد. ما آن را از طریق pip نصب می‌کنیم.
# فلگ --break-system-packages برای توزیع‌های جدید Alpine لازم است
RUN pip3 install --no-cache-dir waymore --break-system-packages

WORKDIR /root/

# --- کپی ابزارها ---
COPY --from=go_builder /go/bin/subfinder /usr/local/bin/subfinder
COPY --from=go_builder /go/bin/assetfinder /usr/local/bin/assetfinder
COPY --from=go_builder /go/bin/alterx /usr/local/bin/alterx
COPY --from=go_builder /go/bin/dnsx /usr/local/bin/dnsx
COPY --from=go_builder /go/bin/cero /usr/local/bin/cero
COPY --from=go_builder /go/bin/puredns /usr/local/bin/puredns
COPY --from=go_builder /out/massdns /usr/local/bin/massdns
COPY --from=go_builder /go/bin/httpx /usr/local/bin/httpx
COPY --from=go_builder /go/bin/cdncheck /usr/local/bin/cdncheck
COPY --from=go_builder /go/bin/nuclei /usr/local/bin/nuclei
COPY --from=go_builder /go/bin/waybackurls /usr/local/bin/waybackurls
COPY --from=go_builder /go/bin/gau /usr/local/bin/gau
COPY --from=go_builder /go/bin/katana /usr/local/bin/katana

# کپی برنامه اصلی
COPY --from=go_builder /app/hunt-engine-api .

# کپی اسکریپت‌های کمکی (مثل AbuseDB)
COPY --from=go_builder /app/scripts /root/hunt-engine/backend/scripts/
RUN chmod +x /root/hunt-engine/backend/scripts/*.sh

# Fetch latest amass release URL from GitHub API
RUN AMASS_URL=$(wget -qO- https://api.github.com/repos/owasp-amass/amass/releases/latest | grep "browser_download_url.*linux_amd64.tar.gz" | cut -d '"' -f 4) && \
    mkdir -p /tmp/amass_extracted && \
    wget -qO /tmp/amass.tar.gz "$AMASS_URL" && \
    tar -xzf /tmp/amass.tar.gz -C /tmp/amass_extracted && \
    mv /tmp/amass_extracted/amass_linux_amd64/amass /usr/local/bin/amass && \
    rm -rf /tmp/amass.tar.gz /tmp/amass_extracted

# دانلود وردلیست‌ها
RUN set -eu; \
    rm -rf /root/nuclei-templates; \
    mkdir -p /root/nuclei-templates /data/nuclei/custom; \
    wget -qO /tmp/nuclei-templates.tar.gz https://github.com/projectdiscovery/nuclei-templates/archive/refs/heads/main.tar.gz; \
    tar -xzf /tmp/nuclei-templates.tar.gz -C /tmp; \
    src="$(find /tmp -maxdepth 1 -type d -name 'nuclei-templates-*' | head -n 1)"; \
    test -n "$src"; \
    cp -a "$src"/. /root/nuclei-templates/; \
    rm -rf /tmp/nuclei-templates.tar.gz "$src"; \
    count="$(find /root/nuclei-templates -type f \( -name '*.yaml' -o -name '*.yml' \) | wc -l)"; \
    echo "Nuclei template count: $count"; \
    test "$count" -gt 0

RUN mkdir -p /wordlists
RUN curl -sL https://raw.githubusercontent.com/v0re/dirb/master/wordlists/common.txt -o /wordlists/common.txt
RUN curl -sL https://raw.githubusercontent.com/PortSwigger/param-miner/master/resources/params -o /wordlists/params.txt

RUN chmod +x /usr/local/bin/*

EXPOSE 8080
CMD ["./hunt-engine-api"]

# HUNT_NUCLEI_SPLIT_WRAPPER_START
COPY hunt-nuclei-wrapper.sh /usr/local/bin/hunt-nuclei-wrapper.sh
RUN set -eux; \
    real="$(command -v nuclei)"; \
    test -n "$real"; \
    if grep -q "HUNT_NUCLEI_SPLIT_WRAPPER" "$real" 2>/dev/null; then echo "Refusing to wrap an already wrapped nuclei binary"; exit 1; fi; \
    rm -f /usr/local/bin/nuclei.real; \
    cp -L "$real" /usr/local/bin/nuclei.real; \
    chmod 0755 /usr/local/bin/nuclei.real /usr/local/bin/hunt-nuclei-wrapper.sh; \
    ln -sf /usr/local/bin/hunt-nuclei-wrapper.sh /usr/local/bin/nuclei
# HUNT_NUCLEI_SPLIT_WRAPPER_END

