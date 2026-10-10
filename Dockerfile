# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /zpush ./cmd/server/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -S zpush && adduser -S zpush -G zpush

WORKDIR /app

# Copy binary from builder
COPY --from=builder /zpush /app/zpush

# Create data directory
RUN mkdir -p /data && chown zpush:zpush /data

USER zpush

EXPOSE 8080

ENV LISTEN_ADDR=:8080
ENV DATABASE_PATH=/data/zpush.db
ENV LOG_LEVEL=info

ENTRYPOINT ["/app/zpush"]
