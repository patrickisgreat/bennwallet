# ─── Go backend build stage ───────────────────────────────────────────
FROM golang:1.24-bullseye AS build
WORKDIR /src/backend

# speed-up: set a CDN mirror first, fall back to direct Git fetches
RUN go env -w GOPROXY=https://goproxy.io,direct

# copy go.mod/sum so we can cache the download layer
COPY backend/go.* ./

# 1) download modules into a cached mount
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# 2) copy the rest of the source
COPY backend .

# 3) compile, again using the cache mounts
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -a -o /go/bin/bennwallet .

# ─── tiny runtime stage (scratch or alpine) ───────────────────────────
FROM scratch
WORKDIR /app
COPY --from=build /usr/share/ca-certificates /usr/share/ca-certificates
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/bennwallet .
EXPOSE 8080
CMD ["./bennwallet"]
