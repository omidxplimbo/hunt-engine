FROM golang:1.24-bookworm AS builder

WORKDIR /src

COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download

COPY backend/ /src/backend/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/hunt-engine-api ./cmd/server

FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    dnsutils \
    curl \
    git \
    bash \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /root

COPY --from=builder /out/hunt-engine-api /root/hunt-engine-api

EXPOSE 8080

CMD ["/root/hunt-engine-api"]
