# ================================================
# Stage 1: Massdns Builder (برای Puredns)
# ================================================
FROM alpine:3.18 AS massdns_builder
RUN apk add --no-cache git build-base make gcc musl-dev linux-headers
WORKDIR /tmp
RUN git clone https://github.com/blechschmidt/massdns.git . && \
    make && \
    mv bin/massdns /usr/local/bin/massdns

# ================================================
# Stage 2: Golang Builder (همه ابزارهای Go)
# ================================================
FROM golang:1.24-alpine AS go_builder
RUN apk add --no-cache git build-base
WORKDIR /app

# --- نصب ابزارهای شکار (فاز ۱) ---
RUN go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
RUN go install -v github.com/tomnomnom/assetfinder@latest
RUN go install -v github.com/projectdiscovery/alterx/cmd/alterx@latest
RUN go install -v github.com/d3mondev/puredns/v2@latest
# 👇👇👇 ابزار جدید: dnsx (رقیب puredns)
RUN go install -v github.com/projectdiscovery/dnsx/cmd/dnsx@latest

# --- نصب ابزارهای شکار (فاز ۲) ---
RUN go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest

# --- بیلد برنامه اصلی ---
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hunt-engine-api ./cmd/server

# ================================================
# Stage 3: Final Runner (تجمیع همه چیز)
# ================================================
FROM alpine:3.18
LABEL maintainer="Omid Security Researcher"
# نصب jq و bind-tools برای دیباگ و کارهای جانبی مفیده
RUN apk --no-cache add ca-certificates bind-tools jq
WORKDIR /root/

# --- کپی کردن ابزارها ---
COPY --from=massdns_builder /usr/local/bin/massdns /usr/local/bin/massdns
COPY --from=go_builder /go/bin/subfinder /usr/local/bin/subfinder
COPY --from=go_builder /go/bin/assetfinder /usr/local/bin/assetfinder
COPY --from=go_builder /go/bin/alterx /usr/local/bin/alterx
COPY --from=go_builder /go/bin/puredns /usr/local/bin/puredns
# 👇👇👇 کپی کردن dnsx
COPY --from=go_builder /go/bin/dnsx /usr/local/bin/dnsx
COPY --from=go_builder /go/bin/httpx /usr/local/bin/httpx

# کپی کردن Amass (سنگین وزن)
COPY --from=caffix/amass:latest /bin/amass /usr/local/bin/amass

# کپی برنامه اصلی
COPY --from=go_builder /app/hunt-engine-api .

RUN chmod +x /usr/local/bin/*

EXPOSE 8080
CMD ["./hunt-engine-api"]