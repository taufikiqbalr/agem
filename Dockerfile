# =========================================
# 1. Build stage
# =========================================
FROM golang:1.25.1-alpine AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    TZ=Asia/Jakarta

# Install dependencies
RUN apk add --no-cache git tzdata

WORKDIR /app

# Copy dependency files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Install swag and scalar
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs
RUN swag init -g ./cmd/api/main.go

# Build binary from the main entry point
RUN go build -o agem-stable ./cmd/api

# =========================================
# 2. Runtime stage
# =========================================
FROM debian:bookworm-slim

ARG APP_PORT=36017
ENV TZ=Asia/Jakarta
WORKDIR /app

# Install runtime dependencies (timezone & SSL)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*

# Copy built binary from builder stage and copy docs
COPY --from=builder /app/agem-stable .
COPY --from=builder /app/docs /app/docs

# Expose your app port
EXPOSE ${APP_PORT}

# Set User
USER www-data

# Run the app
ENTRYPOINT ["/app/agem-stable"]
