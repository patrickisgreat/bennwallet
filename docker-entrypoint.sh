#!/bin/bash
set -e

# Initialize function for logging
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $1"
}

log "Starting BennWallet application..."

# Check if database connection is available
log "Checking PostgreSQL connection..."

# Initialize database
log "Initializing database..."

# Check if we're using DATABASE_URL or individual params
if [ -n "$DATABASE_URL" ]; then
    log "Using DATABASE_URL for connection"
    export PG_CONN="$DATABASE_URL"
else
    log "Using individual connection parameters"
    export PG_CONN="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}"
fi

# Test database connection
RETRIES=5
for i in $(seq 1 $RETRIES); do
    log "Testing database connection (attempt $i/$RETRIES)..."
    if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USER" -p "$DB_PORT" -c "SELECT 1" postgres &>/dev/null; then
        log "Database connection successful!"
        break
    fi
    
    if [ $i -eq $RETRIES ]; then
        log "Cannot connect to PostgreSQL. Please check your connection settings."
        # In DO, we don't exit, we just continue since the app will retry
    else
        log "Connection failed, retrying in 5 seconds..."
        sleep 5
    fi
done

# Run database migrations automatically on startup
# This is triggered by the GitHub Action, so we don't need to do it here
# We'll expose the commands for GitHub Actions to call

# Execute the provided command (usually the app)
exec "$@" 