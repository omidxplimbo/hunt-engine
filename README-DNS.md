# DNS Server Setup - راهنمای سریع

## خلاصه

حالا شما می‌توانید خودتان DNS Server باشید! نیازی به تنظیم DNS در پنل خارجی ندارید.

## مراحل سریع

### 1. تنظیم `.env`

```bash
cp .env.example .env
nano .env
```

```env
DOMAIN_NAME=yourdomain.com
SSL_EMAIL=admin@yourdomain.com
SERVER_IP=YOUR_SERVER_PUBLIC_IP
```

### 2. باز کردن پورت DNS

```bash
sudo ufw allow 53/udp
sudo ufw allow 53/tcp
```

### 3. راه‌اندازی DNS Server

```bash
docker-compose build dns
docker-compose up -d dns
```

### 4. تنظیم Name Servers در Registrar

در پنل Registrar دامنه خود:
- Name Server 1: `ns1.yourdomain.com`
- Name Server 2: `ns2.yourdomain.com`

⚠️ ممکن است نیاز باشد Glue Records اضافه کنید:
- `ns1.yourdomain.com` → `YOUR_SERVER_IP`
- `ns2.yourdomain.com` → `YOUR_SERVER_IP`

### 5. راه‌اندازی بقیه سرویس‌ها

```bash
docker-compose build frontend
docker-compose up -d
./init-ssl.sh
```

## مستندات کامل

برای اطلاعات بیشتر، فایل `DNS-SETUP.md` را مطالعه کنید.

## تست DNS

```bash
# تست محلی
dig @127.0.0.1 yourdomain.com

# تست از خارج
dig @YOUR_SERVER_IP yourdomain.com
```
