#!/bin/bash

# Try all possible database locations
DB_PATHS=(
  "../transactions.db"
  "../bennwallet.db"
  "../backend/transactions.db"
  "../backend/database.db"
)

# Load the test data to all databases
for DB_FILE in "${DB_PATHS[@]}"; do
  if [ -f "$DB_FILE" ]; then
    echo "Loading test data to database at $DB_FILE"
    
    echo "Running SQL script..."
    sqlite3 "$DB_FILE" < populate_test_data.sql
    
    echo "Verifying insertion was successful..."
    echo "Transactions entered by Sarah (any case):"
    sqlite3 "$DB_FILE" "SELECT id, amount, description, payTo, enteredBy FROM transactions WHERE enteredBy LIKE '%sarah%' OR enteredBy LIKE '%Sarah%' OR enteredBy LIKE '%SARAH%'"
    
    echo ""
    echo "Transactions with Sarah as payTo (any case):"
    sqlite3 "$DB_FILE" "SELECT id, amount, description, payTo, enteredBy FROM transactions WHERE payTo LIKE '%sarah%' OR payTo LIKE '%Sarah%' OR payTo LIKE '%SARAH%'"
    
    echo ""
    echo "Test data population complete for $DB_FILE!"
    echo "------------------------------------------------"
  fi
done

echo "All database files processed!" 