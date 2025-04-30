###############################################################################
# 1) Build stage — CGO enabled, Debian tool-chain
###############################################################################
FROM golang:1.24-bullseye AS build

WORKDIR /src/backend

# deps first for cache hits
COPY backend/go.* ./
RUN apt-get update && apt-get install -y --no-install-recommends \
      gcc libc6-dev make ca-certificates && \
    go env -w GOPROXY=https://goproxy.io,direct && \
    go mod download

COPY backend .

# compile with CGO (default CGO_ENABLED=1 on Debian images)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /usr/local/bin/bennwallet .

###############################################################################
# 2) Final runtime image — tiny but has libsqlite3
###############################################################################
FROM debian:bookworm-slim

WORKDIR /app

# runtime deps: libsqlite3 & CA bundle
RUN apt-get update && apt-get install -y --no-install-recommends \
      libsqlite3-0 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# mountpoint for the Fly volume
RUN mkdir -p /data

COPY --from=build /usr/local/bin/bennwallet ./bennwallet
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080
ENV PORT=8080
CMD ["./bennwallet"]
