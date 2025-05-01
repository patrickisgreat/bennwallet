#!/bin/bash
# setup_dev.sh - Development setup script for BennWallet

echo "Setting up BennWallet for local development..."

# Check if .env file exists, create if not
if [ ! -f .env ]; then
  echo "Creating template .env file..."
  cat > .env << EOF
# YNAB API settings for User 1 (Sarah)
# Get these from your YNAB account at https://app.youneedabudget.com/settings/developer
YNAB_TOKEN_USER_1=
YNAB_BUDGET_ID_USER_1=
YNAB_ACCOUNT_ID_USER_1=

# YNAB API settings for User 2 (Patrick)
YNAB_TOKEN_USER_2=
YNAB_BUDGET_ID_USER_2=
YNAB_ACCOUNT_ID_USER_2=
EOF
  echo "Created template .env file."
  echo "Please edit .env and add your YNAB credentials before continuing."
  exit 1
fi

# Source the .env file to load variables
echo "Loading variables from .env file..."
set -a # automatically export all variables
source .env
set +a

# Check if .env variables are set
if [ -z "$YNAB_TOKEN_USER_1" ] || [ -z "$YNAB_BUDGET_ID_USER_1" ] || [ -z "$YNAB_ACCOUNT_ID_USER_1" ]; then
  echo "Error: YNAB credentials not configured properly in .env"
  echo "Please edit .env and set YNAB_TOKEN_USER_1, YNAB_BUDGET_ID_USER_1, and YNAB_ACCOUNT_ID_USER_1"
  exit 1
fi

echo "YNAB credentials loaded successfully."

# Initialize the database
echo "Initializing database..."
cd backend
go run cmd/migrate/main.go
cd ..

# Insert or update test users
echo "Setting up test users..."
sqlite3 transactions.db << EOF
INSERT OR IGNORE INTO users (id, username, name, status, isAdmin) 
VALUES ('1', 'sarah', 'Sarah', 'approved', 1);

INSERT OR IGNORE INTO users (id, username, name, status, isAdmin) 
VALUES ('2', 'patrick', 'Patrick', 'approved', 0);
EOF

# Add YNAB settings for both users
echo "Setting up YNAB configurations..."
sqlite3 transactions.db << EOF
INSERT OR REPLACE INTO user_ynab_settings (user_id, token, budget_id, account_id, sync_enabled) 
VALUES ('1', '$YNAB_TOKEN_USER_1', '$YNAB_BUDGET_ID_USER_1', '$YNAB_ACCOUNT_ID_USER_1', 1);

INSERT OR REPLACE INTO user_ynab_settings (user_id, token, budget_id, account_id, sync_enabled) 
VALUES ('2', '$YNAB_TOKEN_USER_2', '$YNAB_BUDGET_ID_USER_2', '$YNAB_ACCOUNT_ID_USER_2', 1);
EOF

# Set execute permissions
chmod +x setup_dev.sh

echo "Local development setup complete!"
echo "Run 'go run backend/main.go' to start the server." 