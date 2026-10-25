# Proxy args (passed via --build-arg, no effect if unset)
ARG HTTP_PROXY
ARG HTTPS_PROXY

# Frontend build stage
FROM node:22-alpine AS web-builder

ARG HTTP_PROXY
ARG HTTPS_PROXY
ENV HTTP_PROXY=${HTTP_PROXY} HTTPS_PROXY=${HTTPS_PROXY}

WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Backend build stage
FROM golang:1.26-alpine AS backend-builder

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
ARG VERSION=dev
ENV HTTP_PROXY=${HTTP_PROXY} HTTPS_PROXY=${HTTPS_PROXY} GOPROXY=${GOPROXY}

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy frontend dist from builder (Go server serves web/dist as SPA root)
COPY --from=web-builder /web/dist /app/web/dist
# Build the server binary (VERSION is injected via ldflags)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X notygo/internal/api.Version=${VERSION}" -o /notygo ./cmd/server/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -S notygo && adduser -S notygo -G notygo

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /notygo /app/notygo

# Copy frontend static files
COPY --from=web-builder /web/dist /app/web/dist

USER notygo

EXPOSE 9003

# Default configuration
# DATABASE_DSN is required and must be set at runtime
ENV LISTEN_ADDR=:9003
ENV LOG_LEVEL=info

ENTRYPOINT ["/app/notygo"]
