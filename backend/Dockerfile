# ================================================
# Stage 1: Massdns Builder (کامپایل ابزار C)
# ================================================
FROM alpine:3.18 AS massdns_builder
RUN apk add --no-cache git build-base make gcc musl-dev linux-headers
WORKDIR /tmp
RUN git clone https://github.com/blechschmidt/massdns.git . && \
    make && \
    mv bin/massdns /usr/local/bin/massdns

# ================================================
# Stage 2: Golang Builder (کامپایل ابزارهای Go و برنامه اصلی)
# ================================================
FROM golang:1.24-alpine AS go_builder
RUN apk add --no-cache git build-base
WORKDIR /app

# --- نصب ابزارهای شکار سریع ---
RUN go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
RUN go install -v github.com/tomnomnom/assetfinder@latest
RUN go install -v github.com/projectdiscovery/alterx/cmd/alterx@latest
RUN go install -v github.com/d3mondev/puredns/v2@latest

# --- بیلد برنامه اصلی ---
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hunt-engine-api ./cmd/server

# ================================================
# Stage 3: Final Runner (ایمیج نهایی)
# ================================================
FROM alpine:3.18
LABEL maintainer="Omid Security Researcher"
RUN apk --no-cache add ca-certificates bind-tools jq # jq برای پردازش JSON در شل مفیده
WORKDIR /root/

# --- کپی کردن ابزارها ---
# 1. ابزارهای کامپایل شده
COPY --from=massdns_builder /usr/local/bin/massdns /usr/local/bin/massdns
COPY --from=go_builder /go/bin/subfinder /usr/local/bin/subfinder
COPY --from=go_builder /go/bin/assetfinder /usr/local/bin/assetfinder
COPY --from=go_builder /go/bin/alterx /usr/local/bin/alterx
COPY --from=go_builder /go/bin/puredns /usr/local/bin/puredns

# 👇👇👇 2. کپی کردن Amass از ایمیج رسمی خودش (سریع‌ترین روش نصب)
COPY --from=caffix/amass:latest /bin/amass /usr/local/bin/amass

# 3. برنامه اصلی
COPY --from=go_builder /app/hunt-engine-api .

# اطمینان از اجرایی بودن و نصب صحیح resolverهای puredns
RUN chmod +x /usr/local/bin/*

EXPOSE 8080
CMD ["./hunt-engine-api"]