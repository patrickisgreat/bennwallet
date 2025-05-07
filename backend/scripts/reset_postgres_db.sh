#!/bin/bash
set -e

echo "Resetting PostgreSQL database..."

# If DATABASE_URL is set, use it directly
if [ -n "$DATABASE_URL" ]; then
  echo "Using DATABASE_URL for connection"
  CONNECTION_STRING="$DATABASE_URL"
else
  # Otherwise use individual components
  echo "Using individual connection parameters"
  
  # Make sure required variables are set
  if [ -z "$DB_HOST" ] || [ -z "$DB_PORT" ] || [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
    echo "ERROR: Database connection parameters not set. Please set DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME"
    exit 1
  fi
  
  CONNECTION_STRING="postgresql://$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=$DB_SSL_MODE"
fi

# Drop and recreate all tables
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

echo "All tables dropped. The application will recreate them on next startup."
echo "Set the RESET_DB=true environment variable to seed default data."

exit 0 