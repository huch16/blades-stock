# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    git \
    nodejs \
    npm \
    python3 \
    make \
    && npm install -g wails@latest

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build frontend
WORKDIR /app/frontend
RUN npm ci && npm run build

# Build Go binary
WORKDIR /app
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -tags "desktop,production" \
    -ldflags "-w -s -H windowsgui" \
    -o /app/blades-stock .

# Runtime stage
FROM alpine:latest

# Install runtime dependencies (sqlite, etc.)
RUN apk add --no-cache \
    ca-certificates \
    sqlite-libs \
    tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/blades-stock .

# Create data directory
RUN mkdir -p /app/data /app/logs

# Expose metrics port
EXPOSE 9464

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9464/healthz || exit 1

# Run as non-root user
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

USER appuser

ENTRYPOINT ["./blades-stock"]
