#!/bin/bash

# Test Data Seeding Script for Local Development
# ===============================================
# 
# This script seeds test data directly to a local PostgreSQL database
# using connection details from config/config.yaml

set -e

echo "🔍 Reading database connection from config/config.yaml..."

# Parse YAML using sed to extract database section, then grep specific fields
DB_SECTION=$(sed -n '/^database:/,/^[a-z]/p' config/config.yaml)

DB_HOST=$(echo "$DB_SECTION" | grep "host:" | awk '{print $2}' | tr -d '"')
DB_PORT=$(echo "$DB_SECTION" | grep "port:" | awk '{print $2}' | tr -d '"')
DB_USER=$(echo "$DB_SECTION" | grep "user:" | awk '{print $2}' | tr -d '"')
DB_PASSWORD=$(echo "$DB_SECTION" | grep "password:" | awk '{print $2}' | tr -d '"')
DB_NAME=$(echo "$DB_SECTION" | grep "dbname:" | awk '{print $2}' | tr -d '"')

echo "📊 Database connection details:"
echo "   Host: $DB_HOST"
echo "   Port: $DB_PORT"
echo "   Database: $DB_NAME"
echo "   User: $DB_USER"

# Validation
if [ -z "$DB_HOST" ] || [ -z "$DB_USER" ] || [ -z "$DB_NAME" ]; then
    echo "❌ Error: Could not parse database config from config/config.yaml"
    echo ""
    echo "Expected format:"
    echo "database:"
    echo "  host: \"localhost\""
    echo "  port: 5432"
    echo "  user: \"<user>\""
    echo "  password: \"<password>\""
    echo "  dbname: \"<dbname>"
    exit 1
fi

# Check if SQL file exists
SQL_FILE="$(dirname "$0")/test-data.sql"

if [ ! -f "$SQL_FILE" ]; then
    echo "❌ SQL file not found: $SQL_FILE"
    exit 1
fi

echo "📝 Applying test data from $SQL_FILE..."
echo ""

# Try to detect if PostgreSQL is running in Docker
if docker ps --format '{{.Names}}' | grep -q postgres 2>/dev/null; then
    echo "🐳 Detected PostgreSQL running in Docker"
    POSTGRES_CONTAINER=$(docker ps --format '{{.Names}}' | grep postgres | head -1)
    echo "   Using container: $POSTGRES_CONTAINER"
    
    # Copy SQL file to container and execute with correct user
    docker cp "$SQL_FILE" "$POSTGRES_CONTAINER:/tmp/test-data.sql"
    
    if ! docker exec -i "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -f /tmp/test-data.sql; then
        echo ""
        echo "⚠️  Note: If you see 'role does not exist', the Docker container might use a different user."
        echo "   Try checking: docker exec $POSTGRES_CONTAINER psql -U postgres -d $DB_NAME -f /tmp/test-data.sql"
        exit 1
    fi
    
elif command -v psql &> /dev/null; then
    echo "💻 Using local psql client"
    
    # Execute SQL file directly with psql
    if ! PGPASSWORD="$DB_PASSWORD" psql \
        -h "$DB_HOST" \
        -p "$DB_PORT" \
        -U "$DB_USER" \
        -d "$DB_NAME" \
        -f "$SQL_FILE"; then
        echo "❌ Failed to execute SQL file"
        exit 1
    fi
else
    echo "❌ Error: Cannot find PostgreSQL client"
    echo ""
    echo "Options:"
    echo "  1. If PostgreSQL is in Docker, make sure the container is running:"
    echo "     docker ps | grep postgres"
    echo ""
    echo "  2. Install psql client:"
    echo "     brew install postgresql  # macOS"
    echo "     apt-get install postgresql-client  # Linux"
    echo ""
    echo "  3. Or manually run the SQL file:"
    echo "     docker exec -i $POSTGRES_CONTAINER psql -U $DB_USER -d $DB_NAME -f /tmp/test-data.sql"
    exit 1
fi

echo ""
echo "✅ Test data inserted successfully!"
echo ""
echo "Verifying data..."
    
    # Count test runs
    if docker ps --format '{{.Names}}' | grep -q postgres 2>/dev/null; then
        POSTGRES_CONTAINER=$(docker ps --format '{{.Names}}' | grep postgres | head -1)
        TEST_RUN_COUNT=$(docker exec -i "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM test_runs;")
    elif command -v psql &> /dev/null; then
        TEST_RUN_COUNT=$(PGPASSWORD="$DB_PASSWORD" psql \
            -h "$DB_HOST" \
            -p "$DB_PORT" \
            -U "$DB_USER" \
            -d "$DB_NAME" \
            -t -c "SELECT COUNT(*) FROM test_runs;")
    fi
    
    echo "   Test runs created: $(echo "$TEST_RUN_COUNT" | xargs)"
