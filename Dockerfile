# Frontend build stage
FROM node:22-alpine AS frontend-builder

WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Backend build stage
FROM golang:1.26-alpine AS backend-builder

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

# Create data directory
RUN mkdir -p /data && chown zpush:zpush /data

USER zpush

EXPOSE 8080

ENV LISTEN_ADDR=:8080
ENV DATABASE_PATH=/data/zpush.db
ENV LOG_LEVEL=info

ENTRYPOINT ["/app/zpush"]
