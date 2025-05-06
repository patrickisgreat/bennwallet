#!/bin/bash
# reset_dev_db.sh - Reset and reseed the local development database

# Make sure we're not in production
if [ "$APP_ENV" = "production" ]; then
  echo "Error: This script should not be run in production!"
  exit 1
fi

echo "Resetting development database..."

# Stop the server if it's running
if pgrep -f "go run backend/main.go" > /dev/null; then
  echo "Stopping running server..."
  pkill -f "go run backend/main.go"
  sleep 2
fi

# Remove the database file and all related files
echo "Removing existing database files..."
rm -f ./database.db
rm -f ./database.db-wal
rm -f ./database.db-shm

echo "Starting the app to create a fresh database with test data..."
cd "$(dirname "$0")"

# Export environment variables
export APP_ENV=development
export RESET_DB=true

# Run the app temporarily to create the database
echo "Running backend with RESET_DB=true to create a fresh database..."
go run backend/main.go &
APP_PID=$!

# Give it time to initialize
echo "Waiting for database initialization (5 seconds)..."
sleep 5

# Verify database was created
if [ ! -f ./database.db ]; then
  echo "ERROR: Database file was not created!"
  if ps -p $APP_PID > /dev/null; then
    kill $APP_PID
  fi
  exit 1
fi

# Perform additional integrity checks on the database
echo "Running database integrity checks..."

# Fix user IDs
echo "Checking for NULL user IDs..."
NULL_USER_IDS=$(sqlite3 ./database.db "SELECT COUNT(*) FROM users WHERE id IS NULL;")
if [ "$NULL_USER_IDS" -gt 0 ]; then
  echo "WARNING: Found $NULL_USER_IDS users with NULL IDs. Fixing..."
  sqlite3 ./database.db "UPDATE users SET id = 'user_' || ROWID WHERE id IS NULL;"
fi

# Fix permission IDs
echo "Checking for NULL permission IDs..."
NULL_PERM_IDS=$(sqlite3 ./database.db "SELECT COUNT(*) FROM permissions WHERE id IS NULL;")
if [ "$NULL_PERM_IDS" -gt 0 ]; then
  echo "WARNING: Found $NULL_PERM_IDS permissions with NULL IDs. Fixing..."
  # Generate a unique ID for permissions with NULL IDs
  sqlite3 ./database.db "UPDATE permissions SET id = 'perm_' || hex(randomblob(16)) WHERE id IS NULL;"
fi

# Fix NULL references in permissions
echo "Fixing permissions references..."
sqlite3 ./database.db "UPDATE permissions SET owner_user_id = 'admin' WHERE owner_user_id IS NULL;"
sqlite3 ./database.db "UPDATE permissions SET granted_user_id = 'admin' WHERE granted_user_id IS NULL;"
sqlite3 ./database.db "UPDATE permissions SET resource_type = 'transactions' WHERE resource_type IS NULL;"
sqlite3 ./database.db "UPDATE permissions SET permission_type = 'read' WHERE permission_type IS NULL;"

# Show summary of database
echo "Database summary:"
echo "Users:"
sqlite3 ./database.db "SELECT id, username, role FROM users;"
echo "Permissions:"
sqlite3 ./database.db "SELECT id, owner_user_id, granted_user_id, permission_type, resource_type FROM permissions;"

# Check if the app is still running
if ps -p $APP_PID > /dev/null; then
  echo "Database created and seeded successfully!"
  echo "Stopping temporary app instance..."
  kill $APP_PID
  
  # Wait for it to fully stop
  sleep 2
  
  echo "✅ Your development database has been reset and populated with test data."
  echo "✅ You can now start your app as normal."
else
  echo "❌ Error: App crashed during database initialization."
  echo "Check logs for details."
  exit 1
fi 