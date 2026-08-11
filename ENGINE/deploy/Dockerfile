# ============================================================================
# VUKA — production image (Stage 2 / BNR Hackathon 2026)
#
# Single self-contained image: the built React SPA + the Go bridge/ledger
# engine served from ONE port. The engine's VUKA_WEB_DIR makes it serve the
# dashboard at "/", the pitch deck at "/deck", and the API under "/api" —
# no separate static host, no CORS split, tunnel/VPS friendly.
#
#   docker build -t vuka:latest .
#   docker run --rm -p 8080:8080 \
#     -e VUKA_DATABASE_URL=postgres://vuka:changeme@pg:5432/vuka?sslmode=disable \
#     -e VUKA_FX_RWF_KES=9.5 \
#     -e VUKA_CORS_ORIGINS=https://your-domain.example \
#     -e MTN_SIM_MODE=success \
#     vuka:latest
# ============================================================================

# ---- Stage 1: build the React dashboard (Vite -> dist/) --------------------
FROM node:22-alpine AS web

WORKDIR /app
COPY FRONTEND/web-dashboard/package.json FRONTEND/web-dashboard/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY FRONTEND/web-dashboard/ ./
RUN npm run build

# ---- Stage 2: build the Go engine (static binary, no CGO) ------------------
FROM golang:1.25-alpine AS engine

WORKDIR /src
COPY ENGINE/go-bridge-core/go.mod ENGINE/go-bridge-core/go.sum ./
RUN go mod download

COPY ENGINE/go-bridge-core/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/vuka-engine ./cmd/main.go

# ---- Stage 3: runtime (scratch-free alpine, ca-certs for outbound TLS) ------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && addgroup -S vuka && adduser -S -G vuka vuka

WORKDIR /app
COPY --from=engine /out/vuka-engine /usr/local/bin/vuka-engine
COPY --from=web /app/dist /app/dist

USER vuka
EXPOSE 8080 50051

# VUKA_WEB_DIR is baked in; the API still listens on :8080 (REST) and :50051 (gRPC).
ENV VUKA_WEB_DIR=/app/dist \
    VUKA_HTTP_ADDR=:8080 \
    VUKA_GRPC_ADDR=:50051 \
    MTN_SIM_MODE=success \
    VUKA_FX_RWF_KES=9.5 \
    VUKA_ADAPTER=mtn-rw

ENTRYPOINT ["/usr/local/bin/vuka-engine"]