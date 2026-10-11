# Proxy args (passed via --build-arg, no effect if unset)
ARG HTTP_PROXY
ARG HTTPS_PROXY

# Frontend build stage
FROM node:22-alpine AS frontend-builder

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
ENV HTTP_PROXY=${HTTP_PROXY} HTTPS_PROXY=${HTTPS_PROXY} GOPROXY=${GOPROXY}

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy frontend dist from builder
COPY --from=frontend-builder /web/dist /app/web/dist
# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /zpush ./cmd/server/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -S zpush && adduser -S zpush -G zpush

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /zpush /app/zpush

# Copy frontend static files
COPY --from=frontend-builder /web/dist /app/web/dist

USER zpush

EXPOSE 9003

# Default configuration
# DATABASE_DSN is required and must be set at runtime
ENV LISTEN_ADDR=:9003
ENV LOG_LEVEL=info

ENTRYPOINT ["/app/zpush"]
