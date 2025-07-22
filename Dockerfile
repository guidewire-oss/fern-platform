# Build stage
FROM --platform=$BUILDPLATFORM golang:1.24.4-alpine AS builder

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application for the target platform
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-w -s" -o fern-platform cmd/fern-platform/main.go

# Runtime stage
FROM alpine:3.21

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S fern && \
    adduser -u 1001 -S fern -G fern

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/fern-platform ./fern-platform

# Copy configuration files
COPY --from=builder /app/config ./config
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/web ./web

# Change ownership
RUN chown -R fern:fern /app

# Switch to non-root user
USER fern

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./fern-platform", "-config", "config/config.yaml"]