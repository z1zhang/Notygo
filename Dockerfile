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
# Build the server binary (CGO_ENABLED=0 works for both SQLite and MySQL drivers)
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

# Create data directory (for SQLite)
RUN mkdir -p /data && chown zpush:zpush /data

USER zpush

EXPOSE 9003

# Default configuration (SQLite mode)
# For MySQL, set: DB_DRIVER=mysql, DATABASE_DSN=user:pass@tcp(host:3306)/zpush?parseTime=true
ENV LISTEN_ADDR=:9003
ENV DB_DRIVER=sqlite
ENV DATABASE_PATH=/data/zpush.db
ENV LOG_LEVEL=info

ENTRYPOINT ["/app/zpush"]
