# راهنمای سریع تنظیم دامنه

## مراحل سریع

### 1. تنظیم DNS
در پنل DNS دامنه خود، یک A Record اضافه کنید که به IP سرور شما اشاره کند.

```
Type: A
Name: @
Value: [IP سرور شما]
```

### 2. تنظیم فایل .env

```bash
cp .env.example .env
nano .env
```

محتویات:
```env
DOMAIN_NAME=yourdomain.com
SSL_EMAIL=your-email@example.com
```

### 3. Build و راه‌اندازی

```bash
# Build frontend با دامنه جدید
docker-compose build frontend

# راه‌اندازی همه سرویس‌ها
docker-compose up -d

# دریافت گواهینامه SSL
./init-ssl.sh
```

### 4. بررسی

```bash
# بررسی وضعیت سرویس‌ها
docker-compose ps

# بررسی لاگ‌ها
docker-compose logs -f nginx
```

### 5. دسترسی

باز کردن مرورگر و رفتن به: `https://yourdomain.com`

---

**نکته:** اگر می‌خواهید اطلاعات بیشتر بدانید، فایل `DOMAIN-SETUP.md` را مطالعه کنید.
