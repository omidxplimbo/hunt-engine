# راهنمای تنظیم DNS Server شخصی (BIND9)

با این راهنما می‌توانید خودتان DNS Server باشید و نیاز به تنظیمات DNS در پنل دامنه ندارید.

## پیش‌نیازها

1. یک دامنه خریداری شده
2. دسترسی به پنل Registrar دامنه (برای تنظیم Name Servers)
3. سرور با IP عمومی
4. پورت 53 باز در firewall (UDP و TCP)

## مراحل نصب

### 1. تنظیم فایل `.env`

فایل `.env` را ویرایش کنید:

```bash
nano .env
```

محتویات:
```env
DOMAIN_NAME=yourdomain.com
SSL_EMAIL=your-email@example.com
SERVER_IP=YOUR_SERVER_IP
```

**مثال:**
```env
DOMAIN_NAME=example.com
SSL_EMAIL=admin@example.com
SERVER_IP=109.248.160.151
```

**⚠️ مهم:** `SERVER_IP` باید IP عمومی سرور شما باشد. اگر خالی بگذارید، سیستم سعی می‌کند خودش تشخیص دهد اما بهتر است خودتان مشخص کنید.

### 2. باز کردن پورت DNS در Firewall

```bash
# برای UFW (Ubuntu)
sudo ufw allow 53/udp
sudo ufw allow 53/tcp

# برای iptables
sudo iptables -A INPUT -p udp --dport 53 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 53 -j ACCEPT

# برای firewalld (CentOS/RHEL)
sudo firewall-cmd --permanent --add-service=dns
sudo firewall-cmd --reload
```

### 3. Build و راه‌اندازی DNS Server

```bash
# Build DNS Server
docker compose build dns

# راه‌اندازی DNS Server
docker compose up -d dns
```

### 4. بررسی وضعیت DNS Server

```bash
# بررسی لاگ‌ها
docker compose logs dns

# تست DNS محلی
dig @127.0.0.1 yourdomain.com

# تست DNS از خارج (از یک سرور دیگر یا با استفاده از سرویس‌های آنلاین)
dig @YOUR_SERVER_IP yourdomain.com
```

### 5. تنظیم Name Servers در Registrar

حالا باید Name Servers را در پنل Registrar دامنه خود تنظیم کنید:

1. به پنل Registrar دامنه خود بروید
2. بخش "Name Servers" یا "DNS" را پیدا کنید
3. Name Servers را به این صورت تنظیم کنید:

```
ns1.yourdomain.com
ns2.yourdomain.com
```

**مثال:**
- دامنه: `example.com`
- Name Server 1: `ns1.example.com`
- Name Server 2: `ns2.example.com`

**⚠️ نکته مهم:** ممکن است نیاز باشد ابتدا Glue Records یا A Records برای ns1 و ns2 در registrar تنظیم کنید:
- `ns1.yourdomain.com` → `YOUR_SERVER_IP`
- `ns2.yourdomain.com` → `YOUR_SERVER_IP`

### 6. منتظر بمانید تا DNS Propagate شود

پس از تنظیم Name Servers، منتظر بمانید تا DNS propagate شود (معمولاً 24-48 ساعت، اما ممکن است سریع‌تر باشد).

بررسی کنید:
```bash
# بررسی Name Servers
dig NS yourdomain.com

# بررسی A Record
dig yourdomain.com

# بررسی از یک resolver عمومی
dig @8.8.8.8 yourdomain.com
```

### 7. راه‌اندازی بقیه سرویس‌ها

پس از اینکه DNS به درستی کار کرد:

```bash
# Build frontend
docker compose build frontend

# راه‌اندازی همه سرویس‌ها
docker compose up -d

# دریافت گواهینامه SSL
./init-ssl.sh
```

## مدیریت DNS Records

برای اضافه کردن یا تغییر DNS Records، می‌توانید فایل zone را ویرایش کنید:

```bash
# ویرایش zone file
nano dns/zones/db.yourdomain.com
```

سپس DNS Server را restart کنید:

```bash
docker compose restart dns
```

**یا** می‌توانید zone file را دوباره generate کنید:

```bash
docker compose exec dns /usr/local/bin/generate-zone.sh
docker compose exec dns rndc reload
```

## اضافه کردن Subdomain

برای اضافه کردن subdomain، فایل zone را ویرایش کنید:

```bash
nano dns/zones/db.yourdomain.com
```

سپس یک خط اضافه کنید:

```
subdomain    IN      A       YOUR_SERVER_IP
```

بعد از ویرایش:

```bash
# بررسی zone file
docker compose exec dns named-checkzone yourdomain.com /etc/bind/zones/db.yourdomain.com

# Reload DNS
docker compose exec dns rndc reload
```

## ساختار Zone File

Zone file به صورت خودکار generate می‌شود و شامل:

- **SOA Record**: اطلاعات zone
- **NS Records**: Name Servers (ns1 و ns2)
- **A Records**: برای ns1، ns2 و دامنه اصلی
- **Wildcard A Record**: همه subdomain‌ها به IP سرور اشاره می‌کنند

می‌توانید این ساختار را تغییر دهید و records بیشتری اضافه کنید.

## عیب‌یابی

### مشکل: DNS Server شروع نمی‌شود

```bash
# بررسی لاگ‌ها
docker compose logs dns

# بررسی configuration
docker compose exec dns named-checkconf /etc/bind/named.conf

# بررسی zone file
docker compose exec dns named-checkzone yourdomain.com /etc/bind/zones/db.yourdomain.com
```

### مشکل: DNS Query پاسخ نمی‌دهد

1. بررسی کنید پورت 53 باز است:
   ```bash
   netstat -tuln | grep 53
   ```

2. بررسی کنید DNS Server در حال اجرا است:
   ```bash
   docker compose ps dns
   ```

3. تست محلی:
   ```bash
   dig @127.0.0.1 yourdomain.com
   ```

4. تست از خارج:
   ```bash
   dig @YOUR_SERVER_IP yourdomain.com
   ```

### مشکل: Name Servers propagate نشده

- منتظر بمانید (تا 48 ساعت)
- TTL را کاهش دهید
- بررسی کنید Name Servers درست تنظیم شده‌اند

## مزایای DNS Server شخصی

✅ کنترل کامل روی DNS Records
✅ بدون نیاز به پنل DNS خارجی
✅ امکان اضافه کردن records دلخواه
✅ مدیریت مستقیم zone files
✅ سرعت بالا (بدون dependency به سرویس‌های خارجی)

## نکات امنیتی

- Firewall را تنظیم کنید (فقط پورت‌های لازم باز باشند)
- Regular backups از zone files بگیرید
- Monitoring و logging را فعال نگه دارید
- Regular updates انجام دهید

## پشتیبانی

در صورت بروز مشکل:

```bash
# لاگ‌های DNS
docker compose logs -f dns

# بررسی وضعیت
docker compose ps
```
