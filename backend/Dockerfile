# ================================================
# Stage 1: Massdns Builder (برای Puredns)
# ================================================
FROM alpine:3.18 AS massdns_builder
RUN apk add --no-cache git build-base make gcc musl-dev linux-headers
WORKDIR /tmp
# دانلود و کامپایل massdns (قلب تپنده puredns)
RUN git clone https://github.com/blechschmidt/massdns.git . && \
    make && \
    # انتقال فایل باینری ساخته شده به یک مسیر مشخص
    mv bin/massdns /usr/local/bin/massdns


# ================================================
# Stage 2: Golang Builder (کامپایل ابزارهای Go و برنامه اصلی)
# ================================================
FROM golang:1.24-alpine AS go_builder

# نصب پکیج‌های ضروری برای بیلد
RUN apk add --no-cache git build-base

WORKDIR /app

# --- نصب ابزارهای شکار (Recon Tools) ---

# فاز ۱: کشف
RUN go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
RUN go install -v github.com/tomnomnom/assetfinder@latest
RUN go install -v github.com/projectdiscovery/alterx/cmd/alterx@latest
# نکته: در نسخه جدید puredns، مسیر cmd حذف شده است
RUN go install -v github.com/d3mondev/puredns/v2@latest
RUN go install -v github.com/projectdiscovery/dnsx/cmd/dnsx@latest

# فاز ۲: پروبینگ
RUN go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest


# --- بیلد برنامه اصلی (Backend API) ---
COPY go.mod go.sum ./
# دانلود وابستگی‌ها
RUN go mod download

COPY . .
# CGO_ENABLED=0: ساخت باینری کاملاً استاتیک
# -ldflags="-s -w": حذف اطلاعات دیباگ برای کاهش حجم
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hunt-engine-api ./cmd/server


# ================================================
# Stage 3: Final Runner (ایمیج نهایی و سبک)
# ================================================
FROM alpine:3.18

LABEL maintainer="Omid Security Researcher"

# نصب نیازمندی‌های پایه در زمان اجرا
# ca-certificates: برای HTTPS
# bind-tools: برای دستوراتی مثل dig (گاهی لازمه)
# jq: برای پردازش JSON در شل مفیده
RUN apk --no-cache add ca-certificates bind-tools jq

WORKDIR /root/

# --- کپی کردن ابزارها از مرحله‌های قبل ---

# 1. کپی massdns از مرحله اول
COPY --from=massdns_builder /usr/local/bin/massdns /usr/local/bin/massdns

# 2. کپی ابزارهای Go از مرحله دوم
COPY --from=go_builder /go/bin/subfinder /usr/local/bin/subfinder
COPY --from=go_builder /go/bin/assetfinder /usr/local/bin/assetfinder
COPY --from=go_builder /go/bin/alterx /usr/local/bin/alterx
COPY --from=go_builder /go/bin/puredns /usr/local/bin/puredns
COPY --from=go_builder /go/bin/dnsx /usr/local/bin/dnsx
COPY --from=go_builder /go/bin/httpx /usr/local/bin/httpx

# 3. کپی کردن Amass از ایمیج رسمی خودش (سریع‌ترین روش نصب)
# نکته: نام مخزن Amass به caffix تغییر کرده است
COPY --from=caffix/amass:latest /bin/amass /usr/local/bin/amass

# 4. کپی برنامه اصلی خودمون
COPY --from=go_builder /app/hunt-engine-api .

# 👇👇👇 فیکس نهایی Puredns: ساخت فایل ریزالور ثابت و مطمئن
# این فایل همیشه در مسیر /root/trusted_resolvers.txt در دسترس خواهد بود
RUN echo -e "1.1.1.1\n8.8.8.8\n9.9.9.9\n1.0.0.1" > /root/trusted_resolvers.txt

# اطمینان از اجرایی بودن فایل‌ها
RUN chmod +x /usr/local/bin/*

EXPOSE 8080

CMD ["./hunt-engine-api"]