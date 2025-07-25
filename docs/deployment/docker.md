# Docker Deployment Guide

This guide covers running Fern Platform using Docker containers.

> **Note**: Docker images are currently being published. This guide will be fully functional once v0.1.0 images are available on:
> - GitHub Container Registry: `ghcr.io/guidewire-oss/fern-platform`
> - Docker Hub: `docker.io/guidewireoss/fern-platform`

## Quick Start

### Prerequisites

- Docker Engine 20.10 or later
- PostgreSQL 14+ database
- Redis 6+ instance

### Basic Docker Run

```bash
docker run -d \
  --name fern-platform \
  -p 8080:8080 \
  -e DB_HOST=postgres \
  -e DB_PORT=5432 \
  -e DB_USER=postgres \
  -e DB_PASSWORD=yourpassword \
  -e DB_NAME=fern_platform \
  -e REDIS_HOST=redis \
  -e REDIS_PORT=6379 \
  ghcr.io/guidewire-oss/fern-platform:latest
```

## Docker Compose Setup

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: fern_platform
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    command: redis-server --save 60 1 --loglevel warning
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  fern-platform:
    image: ghcr.io/guidewire-oss/fern-platform:latest
    # Or use Docker Hub: docker.io/guidewireoss/fern-platform:latest
    ports:
      - "8080:8080"
    environment:
      # Database
      DB_HOST: postgres
      DB_PORT: 5432
      DB_USER: postgres
      DB_PASSWORD: postgres
      DB_NAME: fern_platform
      DB_SSLMODE: disable
      
      # Redis
      REDIS_HOST: redis
      REDIS_PORT: 6379
      
      # Server
      SERVER_HOST: 0.0.0.0
      SERVER_PORT: 8080
      
      # Logging
      LOG_LEVEL: info
      LOG_FORMAT: json
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  postgres_data:
  redis_data:
```

Run with Docker Compose:

```bash
# Start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f fern-platform

# Stop all services
docker-compose down
```

## Configuration

### Environment Variables

#### Required Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PORT` | PostgreSQL port | 5432 |
| `DB_USER` | PostgreSQL user | postgres |
| `DB_PASSWORD` | PostgreSQL password | postgres |
| `DB_NAME` | Database name | fern_platform |
| `REDIS_HOST` | Redis host | localhost |
| `REDIS_PORT` | Redis port | 6379 |

#### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_SSLMODE` | PostgreSQL SSL mode | disable |
| `SERVER_PORT` | Server port | 8080 |
| `SERVER_HOST` | Server bind address | 0.0.0.0 |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | info |
| `LOG_FORMAT` | Log format (json, text) | json |
| `AUTH_ENABLED` | Enable authentication | false |

### OAuth Configuration (Optional)

To enable OAuth authentication:

```bash
docker run -d \
  --name fern-platform \
  -p 8080:8080 \
  -e DB_HOST=postgres \
  -e DB_USER=postgres \
  -e DB_PASSWORD=yourpassword \
  -e AUTH_ENABLED=true \
  -e AUTH_OAUTH_ENABLED=true \
  -e AUTH_OAUTH_CLIENT_ID=your-client-id \
  -e AUTH_OAUTH_CLIENT_SECRET=your-client-secret \
  -e AUTH_OAUTH_AUTH_URL=https://auth.example.com/oauth2/authorize \
  -e AUTH_OAUTH_TOKEN_URL=https://auth.example.com/oauth2/token \
  -e AUTH_OAUTH_USER_INFO_URL=https://auth.example.com/oauth2/userinfo \
  -e AUTH_OAUTH_ISSUER_URL=https://auth.example.com \
  -e AUTH_OAUTH_REDIRECT_URL=http://localhost:8080/auth/callback \
  ghcr.io/guidewire-oss/fern-platform:latest
```

## Production Considerations

### 1. Use External Database

For production, use a managed PostgreSQL service:

```bash
docker run -d \
  --name fern-platform \
  -p 8080:8080 \
  -e DB_HOST=your-rds-instance.amazonaws.com \
  -e DB_USER=fern_user \
  -e DB_PASSWORD="${DB_PASSWORD}" \
  -e DB_NAME=fern_production \
  -e DB_SSLMODE=require \
  ghcr.io/guidewire-oss/fern-platform:latest
```

### 2. Enable TLS

Mount certificates and configure TLS:

```bash
docker run -d \
  --name fern-platform \
  -p 8443:8443 \
  -v /path/to/certs:/certs:ro \
  -e SERVER_PORT=8443 \
  -e SERVER_TLS_ENABLED=true \
  -e SERVER_TLS_CERT_FILE=/certs/tls.crt \
  -e SERVER_TLS_KEY_FILE=/certs/tls.key \
  ghcr.io/guidewire-oss/fern-platform:latest
```

### 3. Resource Limits

Set appropriate resource limits:

```bash
docker run -d \
  --name fern-platform \
  --memory="2g" \
  --cpus="2" \
  -p 8080:8080 \
  ghcr.io/guidewire-oss/fern-platform:latest
```

### 4. Monitoring

Enable Prometheus metrics:

```bash
docker run -d \
  --name fern-platform \
  -p 8080:8080 \
  -p 9090:9090 \
  -e MONITORING_METRICS_ENABLED=true \
  -e MONITORING_METRICS_PORT=9090 \
  ghcr.io/guidewire-oss/fern-platform:latest
```

## Troubleshooting

### Check Health Status

```bash
# Using curl
curl http://localhost:8080/health

# Using Docker
docker exec fern-platform wget -qO- http://localhost:8080/health
```

### View Logs

```bash
# Follow logs
docker logs -f fern-platform

# Last 100 lines
docker logs --tail 100 fern-platform

# With timestamps
docker logs -t fern-platform
```

### Database Connection Issues

1. Verify PostgreSQL is accessible:
   ```bash
   docker exec fern-platform pg_isready -h $DB_HOST -p $DB_PORT
   ```

2. Check network connectivity:
   ```bash
   docker exec fern-platform ping postgres
   ```

3. Verify credentials:
   ```bash
   docker exec fern-platform psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT 1"
   ```

### Common Issues

1. **"dial tcp: lookup postgres: no such host"**
   - Ensure PostgreSQL container is running
   - Check Docker network configuration
   - Use IP address instead of hostname

2. **"pq: password authentication failed"**
   - Verify DB_PASSWORD is correct
   - Check PostgreSQL pg_hba.conf settings
   - Ensure user exists in PostgreSQL

3. **"bind: address already in use"**
   - Another service is using port 8080
   - Change port: `-p 8081:8080`

## Container Registries

Fern Platform images are available from:

- **GitHub Container Registry**: `ghcr.io/guidewire-oss/fern-platform`
- **Docker Hub**: `docker.io/guidewireoss/fern-platform`

Both registries contain identical images. Choose based on your preferences or rate limits.

## Next Steps

- [Configure OAuth Authentication](../configuration/oauth.md)
- [Set Up Client Libraries](../developers/integration-guide.md)
- [Production Deployment Guide](./production.md)