# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o sso-switch \
    ./cmd/sso-switch

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 sso && \
    adduser -D -u 1000 -G sso sso

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/sso-switch .

# Create directories for config and certs
RUN mkdir -p /etc/sso-switch/certs && \
    chown -R sso:sso /etc/sso-switch

# Switch to non-root user
USER sso

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./sso-switch"]
CMD ["--config", "/etc/sso-switch/config.yaml"]
