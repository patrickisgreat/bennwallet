# ──────────────────────────────────────────────
# 1) Build the Go backend
# ──────────────────────────────────────────────
FROM golang:1.24 AS backend
WORKDIR /src/backend

# copy the Go module files first for better layer-level caching
COPY backend/go.* ./
RUN go mod download

# now bring in the rest of the backend source
COPY backend .

# build main.go that lives right here (note the final output path)
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api .

# ──────────────────────────────────────────────
# 2) Build the React frontend with Vite
# ──────────────────────────────────────────────
FROM node:20-alpine AS frontend
WORKDIR /app

# install JS deps with layer caching
COPY package*.json .
# if you have a pnpm-lock.yaml or yarn.lock, copy that instead
RUN npm ci

# copy the rest of the frontend (adjust if you keep assets elsewhere)
COPY vite.config.* ./
COPY tsconfig.* ./
COPY src/ ./src/
RUN npm run build         # dist/ will be produced in /app/dist

# ──────────────────────────────────────────────
# 3) Final, tiny runtime image
# ──────────────────────────────────────────────
FROM alpine:3.19
WORKDIR /app

# bring in the compiled Go binary and the static React build
COPY --from=backend   /bin/api  ./api
COPY --from=frontend  /app/dist ./dist

# minimal runtime packages for SSL/TLS support
RUN apk add --no-cache ca-certificates

EXPOSE 8080
ENV PORT=8080

CMD ["./api"]
