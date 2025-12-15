# حل مشکل Port 53

اگر خطای زیر را دیدید:
```
Error: address already in use on port 53
```

## راه حل

از `network_mode: host` استفاده می‌کنیم که به DNS Server اجازه می‌دهد مستقیماً از پورت 53 استفاده کند.

این تنظیمات در `docker-compose.yml` اعمال شده است.

## تست DNS Server

بعد از راه‌اندازی:

```bash
# بررسی وضعیت
docker compose ps dns

# بررسی لاگ‌ها
docker compose logs dns

# تست DNS
dig @127.0.0.1 yourdomain.com

# تست از خارج
dig @YOUR_SERVER_IP yourdomain.com
```

## نکته

وقتی از `network_mode: host` استفاده می‌کنیم، DNS Server مستقیماً روی host network کار می‌کند و نیازی به port mapping نیست.
