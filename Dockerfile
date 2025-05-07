###############################################################################
# 1) Build stage — CGO enabled, Debian tool-chain
###############################################################################
FROM golang:1.24-bullseye AS build

WORKDIR /src/backend

# deps first for cache hits
COPY backend/go.* ./
RUN apt-get update && apt-get install -y --no-install-recommends \
      gcc libc6-dev make ca-certificates libpq-dev && \
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
# 3) Final runtime image — includes PostgreSQL and SQLite clients
###############################################################################
FROM debian:bookworm-slim

WORKDIR /app

# runtime deps: add both PostgreSQL and SQLite clients, and cron for backups
RUN apt-get update && apt-get install -y --no-install-recommends \
      libpq5 libsqlite3-0 ca-certificates sqlite3 postgresql-client cron gzip && \
    rm -rf /var/lib/apt/lists/*
    
# mountpoint for the Fly volume
RUN mkdir -p /data

COPY --from=build /usr/local/bin/bennwallet ./bennwallet
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=frontend /app/dist ./dist

# Copy database initialization script
COPY backend/scripts/init-db.sh /app/init-db.sh
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/init-db.sh /app/docker-entrypoint.sh

EXPOSE 8080
ENV PORT=8080

# Define environment variables with empty defaults
ARG YNAB_API_TOKEN=""
ARG YNAB_BUDGET_ID=""
ARG YNAB_ACCOUNT_ID=""
ARG APP_ENV=""
ARG NODE_ENV=""
ARG RESET_DB=""
ARG PR_DEPLOYMENT=""

# Add PostgreSQL environment variables with defaults
ARG DATABASE_URL=""
ARG DB_HOST="localhost"
ARG DB_PORT="5432"
ARG DB_USER="postgres"
ARG DB_PASSWORD="postgres" 
ARG DB_NAME="bennwallet"
ARG DB_SSL_MODE="disable"

# Pass them through as environment variables
ENV YNAB_API_TOKEN=$YNAB_API_TOKEN
ENV YNAB_BUDGET_ID=$YNAB_BUDGET_ID
ENV YNAB_ACCOUNT_ID=$YNAB_ACCOUNT_ID
ENV APP_ENV=$APP_ENV
ENV NODE_ENV=$NODE_ENV
ENV RESET_DB=$RESET_DB
ENV PR_DEPLOYMENT=$PR_DEPLOYMENT

# PostgreSQL environment variables
ENV DATABASE_URL=$DATABASE_URL
ENV DB_HOST=$DB_HOST
ENV DB_PORT=$DB_PORT
ENV DB_USER=$DB_USER
ENV DB_PASSWORD=$DB_PASSWORD
ENV DB_NAME=$DB_NAME
ENV DB_SSL_MODE=$DB_SSL_MODE

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["./bennwallet"]
