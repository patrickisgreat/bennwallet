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
fi

# Run the application
echo "Starting BennWallet backend..."
exec "$@" 