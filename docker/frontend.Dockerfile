FROM node:20-bookworm-slim AS builder

WORKDIR /src/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./

RUN npm run build

FROM nginx:1.27-alpine AS runtime

COPY docker/nginx.frontend.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/frontend/dist /usr/share/nginx/html

EXPOSE 80
