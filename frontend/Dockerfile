# Stage 1: Build
FROM node:20-alpine as builder
WORKDIR /app

# کپی کردن فایل‌های پکیج برای کش کردن لایه نصب
COPY package.json package-lock.json ./
# نصب دقیق وابستگی‌ها
RUN npm install

# کپی کردن بقیه فایل‌های سورس
COPY . .
# بیلد کردن پروژه برای پروداکشن
RUN npm run build

# Stage 2: Serve with Nginx
FROM nginx:alpine
# کپی کردن خروجی بیلد به مسیر Nginx
COPY --from=builder /app/dist /usr/share/nginx/html

# کانفیگ Nginx برای هندل کردن React Router (SPA)
RUN echo 'server { \
    listen 80; \
    location / { \
        root /usr/share/nginx/html; \
        index index.html index.htm; \
        try_files $uri $uri/ /index.html; \
    } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]