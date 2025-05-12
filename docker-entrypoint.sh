#!/bin/bash
set -e

echo "Starting BennWallet application..."

# PostgreSQL database initialization
if [[ -n "$DATABASE_URL" || -n "$DB_HOST" ]]; then
  echo "Checking PostgreSQL connection..."
  
  # Initialize the database
  /app/init-db.sh
  
  # Set up a health check to wait until PostgreSQL is ready
  max_retries=10
  counter=0
  echo "Waiting for PostgreSQL to be ready..."
  
  while [ $counter -lt $max_retries ]; do
    if [[ -n "$DATABASE_URL" ]]; then
      if PGPASSWORD=$DB_PASSWORD psql "$DATABASE_URL" -c '\q' > /dev/null 2>&1; then
        echo "Successfully connected to PostgreSQL using DATABASE_URL."
        break
      fi
    else
      if PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c '\q' > /dev/null 2>&1; then
        echo "Successfully connected to PostgreSQL using connection parameters."
        break
      fi
    fi
    
    counter=$((counter+1))
    echo "Waiting for PostgreSQL connection... (attempt $counter of $max_retries)"
    sleep 5
  done
  
  if [ $counter -eq $max_retries ]; then
    echo "Failed to connect to PostgreSQL after $max_retries attempts."
    echo "The application will attempt to start anyway, but might fail."
  fi
  
  echo "PostgreSQL setup complete."
  
  # Auto-run migrations only in production, unless disabled
  if [[ "$NODE_ENV" == "production" || "$APP_ENV" == "production" ]] && [[ "$SKIP_AUTO_MIGRATIONS" != "true" ]]; then
    echo "Production environment detected, checking for pending migrations..."
    
    # Run a dry-run first to see if any migrations are pending
    PENDING_MIGRATIONS=$(/app/bennwallet migrate --dry-run | grep -c "would be applied" || true)
    
    if [[ "$PENDING_MIGRATIONS" -gt 0 ]]; then
      echo "Found $PENDING_MIGRATIONS pending migrations. Applying them now..."
      /app/bennwallet migrate
      
      if [ $? -ne 0 ]; then
        echo "⚠️ Warning: Migration failed, but continuing application startup."
        echo "Please check the logs and run migrations manually if needed."
      else
        echo "✅ Migrations completed successfully."
      fi
    else
      echo "✅ Database schema is up to date. No migrations needed."
    fi
  fi
fi

# Run the application
echo "Starting BennWallet backend..."
exec "$@" 