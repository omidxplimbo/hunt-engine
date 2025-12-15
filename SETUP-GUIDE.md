# راهنمای مرحله به مرحله - تنظیم DNS و دامنه

این راهنما دقیقاً به ترتیب مراحل را توضیح می‌دهد. هر مرحله را کامل انجام دهید و بعد به مرحله بعد بروید.

---

## ✅ مرحله 1: آماده‌سازی فایل تنظیمات

### 1.1. ساخت فایل `.env`

```bash
cd /root/hunt-engine
cp .env.example .env
```

### 1.2. ویرایش فایل `.env`

```bash
nano .env
```

### 1.3. پر کردن اطلاعات در `.env`

این مقادیر را وارد کنید:

```env
DOMAIN_NAME=yourdomain.com
SSL_EMAIL=admin@yourdomain.com
SERVER_IP=YOUR_SERVER_PUBLIC_IP
```

**مثال واقعی:**
```env
DOMAIN_NAME=example.com
SSL_EMAIL=admin@example.com
SERVER_IP=1.1.1.1
```

**⚠️ نکات مهم:**
- `DOMAIN_NAME`: دامنه خود را بدون `www` وارد کنید
- `SSL_EMAIL`: ایمیل معتبر وارد کنید (برای Let's Encrypt)
- `SERVER_IP`: IP عمومی سرور خود را وارد کنید

### 1.4. ذخیره و خروج

در `nano`: `Ctrl + X` سپس `Y` سپس `Enter`

---

## ✅ مرحله 2: باز کردن پورت DNS در Firewall

### 2.1. بررسی نوع Firewall

```bash
# برای UFW (Ubuntu)
sudo ufw status

# یا برای firewalld (CentOS/RHEL)
sudo firewall-cmd --list-all
```

### 2.2. باز کردن پورت‌ها

**اگر از UFW استفاده می‌کنید:**

```bash
sudo ufw allow 53/udp
sudo ufw allow 53/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw reload
```

**اگر از firewalld استفاده می‌کنید:**

```bash
sudo firewall-cmd --permanent --add-service=dns
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https
sudo firewall-cmd --reload
```

**اگر از iptables استفاده می‌کنید:**

```bash
sudo iptables -A INPUT -p udp --dport 53 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 53 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 443 -j ACCEPT
sudo iptables-save
```

### 2.3. بررسی باز بودن پورت‌ها

```bash
sudo netstat -tuln | grep -E ':(53|80|443)'
```

باید پورت‌های 53، 80 و 443 را ببینید.

---

## ✅ مرحله 3: ساخت و راه‌اندازی DNS Server

### 3.1. Build کردن DNS Server

```bash
cd /root/hunt-engine
docker compose build dns
```

این مرحله ممکن است چند دقیقه طول بکشد.

### 3.2. راه‌اندازی DNS Server

```bash
docker compose up -d dns
```

### 3.3. بررسی وضعیت DNS Server

```bash
docker compose ps dns
```

باید وضعیت `Up` را ببینید.

### 3.4. بررسی لاگ‌های DNS Server

```bash
docker compose logs dns
```

باید پیام‌های موفقیت آمیز ببینید. اگر خطایی دیدید، آن را یادداشت کنید.

### 3.5. تست DNS Server (محلی)

```bash
# جایگزین کنید yourdomain.com را با دامنه خودتان
dig @127.0.0.1 yourdomain.com
```

باید پاسخ DNS را ببینید. اگر خطا داد، لاگ‌ها را دوباره بررسی کنید.

---

## ✅ مرحله 4: تنظیم Name Servers در Registrar دامنه

### 4.1. ورود به پنل Registrar

به پنل فروشنده دامنه خود بروید (مثلاً Namecheap، GoDaddy، Cloudflare و غیره).

### 4.2. پیدا کردن بخش Name Servers

بخش "Name Servers" یا "DNS Settings" یا "DNS Management" را پیدا کنید.

### 4.3. تنظیم Name Servers

Name Servers را به این صورت تغییر دهید:

```
ns1.yourdomain.com
ns2.yourdomain.com
```

**مثال:**
- اگر دامنه شما `example.com` است:
  - Name Server 1: `ns1.example.com`
  - Name Server 2: `ns2.example.com`

### 4.4. تنظیم Glue Records (اگر نیاز بود)

بعضی Registrarها نیاز به Glue Records دارند. اگر گزینه "Glue Records" یا "Host Records" دیدید:

1. یک Host Record اضافه کنید:
   - Host: `ns1`
   - Type: `A`
   - Value: `YOUR_SERVER_IP` (همان IP که در .env گذاشتید)

2. یک Host Record دیگر اضافه کنید:
   - Host: `ns2`
   - Type: `A`
   - Value: `YOUR_SERVER_IP`

### 4.5. ذخیره تغییرات

تغییرات را Save کنید.

---

## ✅ مرحله 5: منتظر ماندن برای DNS Propagation

### 5.1. زمان انتظار

پس از تنظیم Name Servers، باید منتظر بمانید تا DNS propagate شود:
- معمولاً 15 دقیقه تا 2 ساعت
- در موارد نادر تا 48 ساعت

### 5.2. بررسی DNS Propagation

می‌توانید از این دستورات استفاده کنید:

```bash
# بررسی Name Servers
dig NS yourdomain.com

# بررسی A Record
dig yourdomain.com

# بررسی از یک DNS Server عمومی
dig @8.8.8.8 yourdomain.com
```

یا از سرویس‌های آنلاین:
- https://www.whatsmydns.net/
- https://dnschecker.org/

### 5.3. معیار موفقیت

زمانی که این دستورات IP سرور شما را برگردانند، DNS propagate شده است:

```bash
dig @8.8.8.8 yourdomain.com
```

باید در پاسخ `ANSWER SECTION`، IP سرور خود را ببینید.

---

## ✅ مرحله 6: Build کردن Frontend با دامنه جدید

### 6.1. Build Frontend

```bash
cd /root/hunt-engine
docker compose build frontend
```

این مرحله ممکن است چند دقیقه طول بکشد.

---

## ✅ مرحله 7: راه‌اندازی همه سرویس‌ها

### 7.1. راه‌اندازی سرویس‌ها

```bash
docker compose up -d
```

### 7.2. بررسی وضعیت همه سرویس‌ها

```bash
docker compose ps
```

همه سرویس‌ها باید `Up` باشند:
- backend
- frontend
- nginx
- dns
- postgres
- redis
- certbot

اگر سرویسی `Exit` یا خطا دارد، لاگ‌هایش را بررسی کنید:

```bash
docker compose logs [service-name]
```

---

## ✅ مرحله 8: دریافت گواهینامه SSL

### 8.1. اطمینان از DNS Propagation

قبل از این مرحله، مطمئن شوید DNS propagate شده است (مرحله 5 را بررسی کنید).

### 8.2. اجرای اسکریپت SSL

```bash
cd /root/hunt-engine
./init-ssl.sh
```

### 8.3. بررسی خروجی

این اسکریپت:
- گواهینامه SSL را از Let's Encrypt دریافت می‌کند
- Nginx را با تنظیمات SSL راه‌اندازی می‌کند

اگر خطایی دیدید:
- مطمئن شوید DNS propagate شده است
- پورت 80 باز است
- Nginx در حال اجرا است

### 8.4. بررسی گواهینامه SSL

```bash
ls -la certbot/conf/live/yourdomain.com/
```

باید فایل‌های `fullchain.pem` و `privkey.pem` را ببینید.

---

## ✅ مرحله 9: تست نهایی

### 9.1. تست دسترسی به سایت

در مرورگر به این آدرس بروید:

```
https://yourdomain.com
```

باید سایت شما باز شود و قفل SSL را ببینید.

### 9.2. تست API

```bash
curl https://yourdomain.com/api/
```

باید پاسخ JSON ببینید.

### 9.3. بررسی Redirect HTTP به HTTPS

```bash
curl -I http://yourdomain.com
```

باید `301` یا `302` redirect به HTTPS ببینید.

---

## ✅ مرحله 10: تنظیم تمدید خودکار SSL (اختیاری)

### 10.1. اضافه کردن Cron Job

```bash
crontab -e
```

### 10.2. اضافه کردن خط تمدید SSL

این خط را اضافه کنید:

```cron
0 0 * * * cd /root/hunt-engine && ./renew-ssl.sh >> /var/log/ssl-renewal.log 2>&1
```

این باعث می‌شود هر روز نیمه شب SSL بررسی و در صورت نیاز تمدید شود.

### 10.3. ذخیره و خروج

در `crontab -e`: `Ctrl + X` سپس `Y` سپس `Enter`

---

## 🔍 عیب‌یابی

### مشکل: DNS Server شروع نمی‌شود

```bash
# بررسی لاگ‌ها
docker compose logs dns

# بررسی کانفیگ
docker compose exec dns named-checkconf /etc/bind/named.conf
```

### مشکل: DNS Query پاسخ نمی‌دهد

```bash
# بررسی پورت
sudo netstat -tuln | grep 53

# تست محلی
dig @127.0.0.1 yourdomain.com

# بررسی zone file
docker compose exec dns named-checkzone yourdomain.com /etc/bind/zones/db.yourdomain.com
```

### مشکل: SSL Certificate دریافت نمی‌شود

```bash
# بررسی DNS
dig @8.8.8.8 yourdomain.com

# بررسی پورت 80
sudo netstat -tuln | grep 80

# بررسی لاگ‌های certbot
docker compose logs certbot
```

### مشکل: سایت با IP باز می‌شود اما با دامنه نه

- مطمئن شوید DNS propagate شده است
- بررسی کنید Name Servers درست تنظیم شده‌اند
- چند ساعت صبر کنید

---

## ✅ چک‌لیست نهایی

قبل از اتمام، این موارد را بررسی کنید:

- [ ] فایل `.env` با اطلاعات صحیح پر شده
- [ ] پورت‌های 53، 80 و 443 باز هستند
- [ ] DNS Server در حال اجرا است
- [ ] Name Servers در Registrar تنظیم شده‌اند
- [ ] DNS propagate شده است (از خارج قابل دسترسی است)
- [ ] Frontend build شده است
- [ ] همه سرویس‌ها در حال اجرا هستند
- [ ] گواهینامه SSL دریافت شده است
- [ ] سایت با HTTPS باز می‌شود
- [ ] HTTP به HTTPS redirect می‌شود
- [ ] IP block شده است (فقط دامنه کار می‌کند)

---

## 📞 پشتیبانی

اگر مشکلی پیش آمد:

1. لاگ‌های مربوطه را بررسی کنید:
   ```bash
   docker compose logs -f
   ```

2. وضعیت سرویس‌ها را بررسی کنید:
   ```bash
   docker compose ps
   ```

3. DNS را تست کنید:
   ```bash
   dig @8.8.8.8 yourdomain.com
   ```

---

**موفق باشید! 🎉**
