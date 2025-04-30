# --- build Go backend ---
    FROM golang:1.24 AS backend
    WORKDIR /src/backend
    COPY backend/go.* ./
    RUN go mod download
    COPY backend .
    RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/server
    
    # --- build React frontend ---
    FROM node:20 AS frontend
    WORKDIR /app
    COPY package*.json ./
    RUN npm ci
    COPY src ./src
    RUN npm run build
    
    # --- final image ---
    FROM debian:bookworm-slim
    COPY --from=backend   /bin/api /app/api
    COPY --from=frontend  /app/dist /app/dist
    WORKDIR /app
    ENV PORT=8080
    EXPOSE 8080
    CMD ["./api"]
    