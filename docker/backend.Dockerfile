# syntax=docker/dockerfile:1.7

FROM golang:1.25.7-alpine AS go_builder

RUN apk add --no-cache git build-base

ENV GOPROXY=https://goproxy.io,direct
WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Recon / security tools
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/tomnomnom/assetfinder@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/projectdiscovery/alterx/cmd/alterx@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/projectdiscovery/dnsx/cmd/dnsx@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/glebarez/cero@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/d3mondev/puredns/v2@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/projectdiscovery/cdncheck/cmd/cdncheck@latest

ARG NUCLEI_VERSION=v3.8.0
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/projectdiscovery/nuclei/v3/cmd/nuclei@${NUCLEI_VERSION}

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/tomnomnom/waybackurls@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/lc/gau/v2/cmd/gau@latest
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v github.com/projectdiscovery/katana/cmd/katana@latest
# massdns for puredns
RUN git clone --depth 1 https://github.com/blechschmidt/massdns.git /tmp/massdns \
    && make -C /tmp/massdns \
    && mkdir -p /out \
    && cp /tmp/massdns/bin/massdns /out/massdns

COPY backend/ /src/backend/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/hunt-engine-api ./cmd/server

FROM alpine:3.20 AS runtime

LABEL maintainer="Omid Security Researcher"

RUN apk add --no-cache \
    ca-certificates \
    bind-tools \
    jq \
    curl \
    wget \
    python3 \
    py3-pip \
    git \
    gcc \
    python3-dev \
    musl-dev \
    linux-headers \
    libffi-dev \
    nmap \
    bash \
    unzip \
    chromium \
    tar \
    make

RUN pip3 install --no-cache-dir waymore --break-system-packages

WORKDIR /root

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
COPY --from=go_builder /out/hunt-engine-api /root/hunt-engine-api

COPY backend/scripts /root/hunt-engine/backend/scripts
RUN chmod +x /root/hunt-engine/backend/scripts/*.sh

RUN cp /root/hunt-engine/backend/scripts/hunt-nuclei-wrapper.sh /usr/local/bin/hunt-nuclei-wrapper.sh \
    && chmod +x /usr/local/bin/hunt-nuclei-wrapper.sh \
    && if [ -x /usr/local/bin/nuclei ] && [ ! -e /usr/local/bin/nuclei.real ]; then mv /usr/local/bin/nuclei /usr/local/bin/nuclei.real; fi \
    && ln -sf /usr/local/bin/hunt-nuclei-wrapper.sh /usr/local/bin/nuclei

RUN AMASS_URL=$(wget -qO- https://api.github.com/repos/owasp-amass/amass/releases/latest | grep "browser_download_url.*linux_amd64.tar.gz" | cut -d '"' -f 4) \
    && test -n "$AMASS_URL" \
    && mkdir -p /tmp/amass_extracted \
    && wget -qO /tmp/amass.tar.gz "$AMASS_URL" \
    && tar -xzf /tmp/amass.tar.gz -C /tmp/amass_extracted \
    && find /tmp/amass_extracted -type f -name amass -exec mv {} /usr/local/bin/amass \; \
    && chmod +x /usr/local/bin/amass \
    && rm -rf /tmp/amass.tar.gz /tmp/amass_extracted

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

RUN mkdir -p /wordlists /data/wordlists /data/reports /tmp/hunt-engine \
    && curl -sL https://raw.githubusercontent.com/v0re/dirb/master/wordlists/common.txt -o /wordlists/common.txt \
    && curl -sL https://raw.githubusercontent.com/PortSwigger/param-miner/master/resources/params -o /wordlists/params.txt \
    && chmod +x /usr/local/bin/*

RUN for t in subfinder assetfinder amass alterx cero puredns massdns dnsx httpx waybackurls gau katana nuclei nmap python3 waymore; do \
      command -v "$t" >/dev/null || { echo "missing required runtime tool: $t"; exit 1; }; \
    done

EXPOSE 8080

CMD ["/root/hunt-engine-api"]
