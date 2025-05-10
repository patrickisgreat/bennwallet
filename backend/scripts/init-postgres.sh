#!/bin/bash
set -e

echo "Initializing PostgreSQL database for BennWallet..."

# Create extensions and prepare the database
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Create any needed extensions
    CREATE EXTENSION IF NOT EXISTS pgcrypto;

    -- Set up proper permissions
    GRANT ALL PRIVILEGES ON DATABASE $POSTGRES_DB TO $POSTGRES_USER;
EOSQL

echo "PostgreSQL initialization complete!" 