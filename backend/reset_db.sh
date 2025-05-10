#!/bin/bash
# Script to reset the PostgreSQL database

set -e

# Get the directory of this script
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

# Try to detect if we can use PostgreSQL
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}
DB_NAME=${DB_NAME:-bennwallet}
DB_SSL_MODE=${DB_SSL_MODE:-disable}

# If DATABASE_URL is set, use it directly
if [ -n "$DATABASE_URL" ]; then
    echo "Using DATABASE_URL for database connection"
    CONNECTION_STRING="$DATABASE_URL"
else
    echo "Using individual connection parameters for PostgreSQL"
    CONNECTION_STRING="postgresql://$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=$DB_SSL_MODE"
fi

# Try to connect to PostgreSQL
if command -v psql &> /dev/null; then
    echo "Found PostgreSQL client, attempting to connect..."
    
    if PGPASSWORD=$DB_PASSWORD psql "$CONNECTION_STRING" -c '\l' &> /dev/null; then
        echo "Successfully connected to PostgreSQL database."
        
        echo "Resetting PostgreSQL database..."
        # Run the PostgreSQL reset script
        PGPASSWORD=$DB_PASSWORD psql "$CONNECTION_STRING" <<EOF
-- Drop all tables in the public schema
DO \$\$ 
DECLARE
    r RECORD;
BEGIN
    -- Disable foreign key checks during table deletion
    EXECUTE 'SET CONSTRAINTS ALL DEFERRED';
    
    -- Drop all tables in the public schema
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
    
    -- Re-enable foreign key checks
    EXECUTE 'SET CONSTRAINTS ALL IMMEDIATE';
END \$\$;
EOF
        echo "PostgreSQL database reset complete."
        
        # Start server with RESET_DB flag to populate the database
        echo "Starting server with RESET_DB and --no-exit flag..."
        RESET_DB=true APP_ENV=development go run main.go --no-exit
        
        exit 0
    else
        echo "ERROR: Could not connect to PostgreSQL database."
        echo "Please check your PostgreSQL connection settings."
        exit 1
    fi
else
    echo "ERROR: PostgreSQL client not found."
    echo "Please install the PostgreSQL client (psql) and try again."
    exit 1
fi

echo "Reset complete. You can now start the server normally with 'go run main.go'" 