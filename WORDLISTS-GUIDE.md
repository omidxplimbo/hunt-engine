# راهنمای استفاده از وردلیست‌ها برای Puredns

## محل قرارگیری وردلیست‌ها

### 1. وردلیست‌های پیش‌فرض
وردلیست‌های پیش‌فرض به صورت خودکار در Docker image نصب می‌شوند و در مسیر `/wordlists` داخل container قرار دارند:
- `/wordlists/common.txt` - وردلیست عمومی
- `/wordlists/params.txt` - وردلیست پارامترها

### 2. وردلیست‌های سفارشی
برای اضافه کردن وردلیست‌های سفارشی خود:

1. **ایجاد دایرکتوری** (اگر وجود ندارد):
   ```bash
   mkdir -p /root/hunt-engine/custom_wordlists
   ```

2. **قرار دادن فایل‌های وردلیست** در این دایرکتوری:
   ```bash
   # مثال: کپی کردن یک وردلیست
   cp /path/to/your/wordlist.txt /root/hunt-engine/custom_wordlists/subdomains.txt
   ```

3. **فرمت فایل‌ها**:
   - هر خط یک کلمه (subdomain prefix)
   - خطوط خالی و خطوطی که با `#` شروع می‌شوند نادیده گرفته می‌شوند
   - مثال:
     ```
     api
     www
     admin
     test
     dev
     staging
     ```

4. **بعد از اضافه کردن فایل‌ها**:
   - فایل‌ها به صورت خودکار در container mount می‌شوند (از طریق docker-compose volume)
   - نیازی به restart کردن container نیست
   - می‌توانید در فرانت، در بخش ایجاد/ویرایش Target، وردلیست‌های جدید را ببینید

## استفاده در فرانت

1. هنگام ایجاد یا ویرایش Target:
   - checkbox "PUREDNS" را فعال کنید
   - لیست وردلیست‌های موجود نمایش داده می‌شود
   - وردلیست‌های مورد نظر را انتخاب کنید (می‌توانید چندتایی انتخاب کنید)

2. **نکات مهم**:
   - فقط subdomain‌های **لایو** (resolve شده) ذخیره می‌شوند
   - نتایج puredns با source "puredns" برچسب می‌خورند
   - نتایج با سایر ابزارها (subfinder, assetfinder, etc.) merge و unique می‌شوند

## مثال وردلیست‌های معروف

می‌توانید از وردلیست‌های معروف استفاده کنید:
- [SecLists](https://github.com/danielmiessler/SecLists) - `Discovery/DNS/subdomains-top1million-5000.txt`
- [Assetnote](https://wordlists.assetnote.io/) - وردلیست‌های مختلف
- [DNS Wordlists](https://github.com/bitquark/dnspop) - وردلیست‌های DNS

## ساختار دایرکتوری

```
/root/hunt-engine/
├── custom_wordlists/          # وردلیست‌های سفارشی شما
│   ├── subdomains.txt
│   ├── common-subdomains.txt
│   └── ...
└── docker-compose.yml         # volume mapping: ./custom_wordlists:/wordlists/custom
```

## بررسی وردلیست‌های موجود

می‌توانید از API endpoint استفاده کنید:
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" https://your-domain/api/wordlists
```

یا در فرانت، هنگام ایجاد/ویرایش Target، لیست وردلیست‌ها به صورت خودکار نمایش داده می‌شود.

