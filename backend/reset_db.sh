#!/bin/bash
# Script to reset the PostgreSQL database

set -e

echo "FUCKKKKK"

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
        
        # Create the base schema first
        echo "Creating base schema..."
        PGPASSWORD=$DB_PASSWORD psql "$CONNECTION_STRING" -f database/db.go <<EOF
-- Create base tables in correct order to handle dependencies
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT DEFAULT 'approved',
    is_admin BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    granted_user_id TEXT NOT NULL REFERENCES users(id),
    owner_user_id TEXT NOT NULL REFERENCES users(id),
    resource_type TEXT NOT NULL,
    permission_type TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(granted_user_id, owner_user_id, resource_type, permission_type)
);

CREATE TABLE IF NOT EXISTS ynab_category_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category_group_id TEXT NOT NULL,
    hidden BOOLEAN DEFAULT false,
    user_id TEXT NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ynab_categories (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category_group_id TEXT NOT NULL,
    hidden BOOLEAN DEFAULT false,
    budget_amount DECIMAL(15,2),
    user_id TEXT NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (category_group_id) REFERENCES ynab_category_groups(id)
);

CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    amount NUMERIC(15,2) NOT NULL,
    description TEXT NOT NULL,
    date TIMESTAMP NOT NULL,
    transaction_date TIMESTAMP,
    type TEXT NOT NULL,
    pay_to TEXT,
    paid BOOLEAN NOT NULL DEFAULT FALSE,
    paid_date TEXT,
    optional BOOLEAN NOT NULL DEFAULT FALSE,
    entered_by TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id),
    note TEXT
);

CREATE TABLE IF NOT EXISTS transaction_categories (
    id SERIAL PRIMARY KEY,
    transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    category_id TEXT NOT NULL REFERENCES ynab_categories(id) ON DELETE CASCADE,
    amount NUMERIC(15,2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(transaction_id, category_id)
);
EOF

        # Start server with RESET_DB flag to populate the database
        echo "Starting server with RESET_DB flag..."
        RESET_DB=true APP_ENV=development go run main.go &
        SERVER_PID=$!

        # Wait for server to start and initialize data
        echo "Waiting for server to initialize data..."
        sleep 15

        # Check if server is still running
        if ! kill -0 $SERVER_PID 2>/dev/null; then
            echo "ERROR: Server failed to start"
            exit 1
        fi

        # Stop the server
        echo "Stopping server..."
        kill $SERVER_PID
        wait $SERVER_PID

        # Verify the foreign key constraints
        echo "Verifying foreign key constraints..."
        PGPASSWORD=$DB_PASSWORD psql "$CONNECTION_STRING" <<EOF
-- Check table structure
\d transaction_categories

-- Check foreign key constraints
SELECT
    tc.table_schema, 
    tc.constraint_name, 
    tc.table_name, 
    kcu.column_name, 
    ccu.table_name AS foreign_table_name,
    ccu.column_name AS foreign_column_name,
    rc.delete_rule
FROM 
    information_schema.table_constraints AS tc 
    JOIN information_schema.key_column_usage AS kcu
      ON tc.constraint_name = kcu.constraint_name
      AND tc.table_schema = kcu.table_schema
    JOIN information_schema.referential_constraints AS rc
      ON tc.constraint_name = rc.constraint_name
    JOIN information_schema.constraint_column_usage AS ccu
      ON ccu.constraint_name = tc.constraint_name
WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name='transaction_categories';

-- Check if there are any transactions with categories
SELECT t.id, t.description, tc.category_id 
FROM transactions t 
JOIN transaction_categories tc ON t.id = tc.transaction_id 
LIMIT 5;

-- Check the actual delete rule for the constraint
SELECT conname, pg_get_constraintdef(oid) 
FROM pg_constraint 
WHERE conrelid = 'transaction_categories'::regclass;
EOF

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