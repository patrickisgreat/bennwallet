#!/bin/bash
set -e

# Script to destroy and recreate the database in production
# Warning: This will delete all data!

DB_APP_NAME="bennwallet-prod-db"
MAIN_APP_NAME="bennwallet"

echo "🚨 WARNING! This script will DESTROY your production database and recreate it from scratch."
echo "All data will be lost. This should only be used when you want a completely fresh start."
echo ""
read -p "Are you sure you want to continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
    echo "Operation cancelled."
    exit 1
fi

echo "🗑️  Destroying existing Postgres database..."
fly apps destroy $DB_APP_NAME --yes

echo "🚀 Creating new Postgres database with Fly Managed Postgres..."
fly postgres create --name $DB_APP_NAME

# Get the connection string for the new database
echo "📝 Getting database connection string..."
CONNECTION_STRING=$(fly postgres connect -a $DB_APP_NAME --postgres-url)

# Update the main app with the new database connection string
echo "🔄 Updating main app with new database connection..."
fly secrets set DATABASE_URL="$CONNECTION_STRING" -a $MAIN_APP_NAME

echo "✅ Successfully recreated database and updated connection string."
echo ""
echo "Now you can deploy your application with:"
echo "  fly deploy -a $MAIN_APP_NAME"
echo ""
echo "The database will be created with the correct schema on first access." 