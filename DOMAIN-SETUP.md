# راهنمای تنظیم دامنه و SSL

این راهنما به شما کمک می‌کند تا دامنه خود را به پروژه متصل کرده و SSL رایگان با Let's Encrypt فعال کنید.

## پیش‌نیازها

1. یک دامنه خریداری شده
2. دسترسی به پنل DNS دامنه (Name Servers)
3. سرور با IP عمومی
4. پورت‌های 80 و 443 باز در firewall

## مراحل نصب

### 1. تنظیم DNS Records

در پنل DNS دامنه خود، یک A Record اضافه کنید:

```
Type: A
Name: @ (یا دامنه اصلی)
Value: IP سرور شما
TTL: 3600 (یا حداقل 300)
```

**مثال:**
- دامنه: `example.com`
- IP سرور: `109.248.160.151`
- Record: `A @ 109.248.160.151`

اگر می‌خواهید از subdomain استفاده کنید (مثلاً `app.example.com`):
```
Type: A
Name: app
Value: IP سرور شما
TTL: 3600
```

### 2. کپی کردن فایل تنظیمات

```bash
cp .env.example .env
```

### 3. ویرایش فایل `.env`

فایل `.env` را باز کرده و دامنه خود را تنظیم کنید:

```bash
nano .env
```

محتویات:
```env
DOMAIN_NAME=yourdomain.com
SSL_EMAIL=your-email@example.com
```

**مثال:**
```env
DOMAIN_NAME=example.com
SSL_EMAIL=admin@example.com
```

### 4. ساخت مجدد Frontend (برای به‌روزرسانی API URL)

```bash
docker-compose build frontend
```

### 4. راه‌اندازی اولیه سرویس‌ها

```bash
docker-compose up -d
```

این کار تمام سرویس‌ها را به جز nginx با SSL راه‌اندازی می‌کند.

### 5. دریافت گواهینامه SSL

پس از اطمینان از اینکه DNS به درستی تنظیم شده و دامنه به IP سرور شما اشاره می‌کند:

```bash
./init-ssl.sh
```

این اسکریپت:
- گواهینامه SSL را از Let's Encrypt دریافت می‌کند
- Nginx را با تنظیمات SSL راه‌اندازی می‌کند

**نکته:** ممکن است چند دقیقه طول بکشد تا DNS propagate شود. می‌توانید با دستور زیر بررسی کنید:

```bash
dig yourdomain.com
# یا
nslookup yourdomain.com
```

### 6. بررسی وضعیت

```bash
docker-compose ps
```

همه سرویس‌ها باید در وضعیت `Up` باشند.

### 7. تست دسترسی

مرورگر را باز کرده و به آدرس `https://yourdomain.com` بروید.

## تمدید خودکار گواهینامه SSL

گواهینامه‌های Let's Encrypt هر 90 روز منقضی می‌شوند. برای تمدید خودکار، یک cron job اضافه کنید:

```bash
crontab -e
```

سپس این خط را اضافه کنید:

```cron
0 0 * * * cd /root/hunt-engine && ./renew-ssl.sh
```

این دستور هر روز نیمه شب بررسی می‌کند و در صورت نیاز گواهینامه را تمدید می‌کند.

## تغییر دامنه

اگر نیاز به تغییر دامنه دارید:

1. فایل `.env` را ویرایش کنید:
   ```env
   DOMAIN_NAME=newdomain.com
   SSL_EMAIL=admin@newdomain.com
   ```

2. DNS جدید را تنظیم کنید

3. Frontend را دوباره build کنید:
   ```bash
   docker-compose build frontend
   ```

4. سرویس‌ها را restart کنید:
   ```bash
   docker-compose down
   docker-compose up -d
   ```

5. گواهینامه SSL جدید دریافت کنید:
   ```bash
   ./init-ssl.sh
   ```

## عیب‌یابی

### مشکل: گواهینامه SSL دریافت نمی‌شود

1. بررسی کنید DNS به درستی تنظیم شده:
   ```bash
   dig yourdomain.com
   ```

2. بررسی کنید پورت 80 باز است:
   ```bash
   netstat -tuln | grep 80
   ```

3. لاگ‌های certbot را بررسی کنید:
   ```bash
   docker-compose logs certbot
   ```

### مشکل: سایت با IP باز می‌شود

اگر سایت با IP باز می‌شود، احتمالاً DNS به درستی تنظیم نشده. بررسی کنید:
- A Record درست است
- TTL به اندازه کافی کوتاه است (300-3600)
- منتظر propagation DNS باشید (تا 48 ساعت)

### مشکل: HTTPS کار نمی‌کند

1. بررسی کنید nginx در حال اجرا است:
   ```bash
   docker-compose ps nginx
   ```

2. لاگ‌های nginx را بررسی کنید:
   ```bash
   docker-compose logs nginx
   ```

3. بررسی کنید گواهینامه SSL موجود است:
   ```bash
   ls -la certbot/conf/live/yourdomain.com/
   ```

## امنیت

- گواهینامه SSL به صورت خودکار تمدید می‌شود
- فقط دامنه تنظیم شده قابل دسترسی است (IP block شده)
- HTTP به HTTPS redirect می‌شود
- Security headers اضافه شده‌اند
- Rate limiting فعال است

## پشتیبانی

در صورت بروز مشکل، لاگ‌های سرویس‌ها را بررسی کنید:

```bash
docker-compose logs -f
```
