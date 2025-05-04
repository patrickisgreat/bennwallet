#!/bin/bash

# Set the database path directly
DB_FILE="../transactions.db"

if [ ! -f "$DB_FILE" ]; then
  # Try the absolute path
  DB_FILE="$(cd .. && pwd)/transactions.db"
  
  if [ ! -f "$DB_FILE" ]; then
    echo "Database file not found at $DB_FILE"
    exit 1
  fi
fi

echo "Using database at $DB_FILE"

# List all tables
echo "Database tables:"
sqlite3 "$DB_FILE" ".tables"

# Show schema for transactions table
echo -e "\nTransactions table schema:"
sqlite3 "$DB_FILE" ".schema transactions"

# Show schema for users table
echo -e "\nUsers table schema:"
sqlite3 "$DB_FILE" ".schema users"

# Count total transactions
echo -e "\nTotal transactions:"
sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM transactions;"

# Count total users
echo -e "\nTotal users:"
sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM users;" 