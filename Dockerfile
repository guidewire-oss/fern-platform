# Web stage: build the v2 SPA so contributors without local Node still
# get a working image. The make target `web-v2-build` does this on
# the host when pnpm is available; this stage is the fallback. Either
# way, `//go:embed all:dist` in internal/web/embed.go captures whatever
# is in internal/web/dist/ at Go-build time.
FROM --platform=$BUILDPLATFORM node:20-alpine AS web
WORKDIR /web
RUN corepack enable
COPY web-v2/package.json web-v2/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web-v2/ ./
RUN pnpm build

# Go build stage
FROM --platform=$BUILDPLATFORM golang:1.24.5-alpine AS builder

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

# Drop the freshly-built SPA into internal/web/dist *before* go build,
# so //go:embed picks up the latest bundle even if the host hasn't run
# `make web-v2-build`.
COPY --from=web /web/dist ./internal/web/dist

# Build the application for the target platform
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-w -s" -o fern-platform cmd/fern-platform/main.go

# Build the load-test seeder. Same image carries both binaries so
# `docker compose run --rm fern /app/seed` works without a second
# image build.
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-w -s" -o seed ./cmd/seed

# Runtime stage
FROM alpine:3.22

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S fern && \
    adduser -u 1001 -S fern -G fern

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/fern-platform ./fern-platform
COPY --from=builder /app/seed ./seed

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