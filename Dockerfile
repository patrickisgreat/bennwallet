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
# 2) Build the React frontend with Vite
###############################################################################
FROM node:20-alpine AS frontend
WORKDIR /app

# install JS deps with layer caching
COPY package*.json ./
RUN npm ci

# copy the rest of the frontend
COPY vite.config.* ./
COPY tsconfig.* ./
COPY index.html ./
COPY src/ ./src/
COPY public/ ./public/
COPY tailwind.config.* ./
COPY postcss.config.* ./
RUN npm run build

###############################################################################
# 3) Final runtime image — tiny but has libsqlite3
###############################################################################
FROM debian:bookworm-slim

WORKDIR /app

# runtime deps: add sqlite3 CLI and cron for backups
RUN apt-get update && apt-get install -y --no-install-recommends \
      libsqlite3-0 ca-certificates sqlite3 cron gzip && \
    rm -rf /var/lib/apt/lists/*
    
# mountpoint for the Fly volume
RUN mkdir -p /data

COPY --from=build /usr/local/bin/bennwallet ./bennwallet
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=frontend /app/dist ./dist

EXPOSE 8080
ENV PORT=8080
# Define YNAB API token and IDs as build args with empty defaults
ARG YNAB_API_TOKEN=""
ARG YNAB_BUDGET_ID=""
ARG YNAB_ACCOUNT_ID=""
# Pass them through as environment variables
ENV YNAB_API_TOKEN=$YNAB_API_TOKEN
ENV YNAB_BUDGET_ID=$YNAB_BUDGET_ID
ENV YNAB_ACCOUNT_ID=$YNAB_ACCOUNT_ID

CMD ["./bennwallet"]
