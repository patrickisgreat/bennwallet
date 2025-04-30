# ──────────────────────────────────────────
# 1) Build the Go backend
# ──────────────────────────────────────────
FROM golang:1.24 AS backend
WORKDIR /src/backend

COPY backend/go.* ./
RUN go mod download

COPY backend .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api .

# ──────────────────────────────────────────
# 2) Build the React frontend with Vite
# ──────────────────────────────────────────
FROM node:20-alpine AS frontend
WORKDIR /app

# deps first for better cache hit-rate
COPY package*.json ./
RUN npm ci --omit=dev     # vite is a prod dep, dev-only deps are skipped

# copy **everything except what .dockerignore filters out**
COPY . .

RUN npm run build         # → /app/dist

# ──────────────────────────────────────────
# 3) Final, minimal runtime image
# ──────────────────────────────────────────
FROM alpine:3.19
WORKDIR /app

# CA certs for outbound HTTPS traffic
RUN apk add --no-cache ca-certificates

COPY --from=backend  /bin/api  ./api
COPY --from=frontend /app/dist ./dist

ENV PORT=8080
EXPOSE 8080

CMD ["./api"]
