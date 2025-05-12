#!/bin/bash
set -e

# Script to completely recreate the database with a clean schema
# This is useful during development or when you need to reset the database

echo "This script will completely destroy and recreate your database."
echo "This will permanently delete all data. Make sure you have backups if needed."
echo "Are you sure you want to continue? (y/N)"
read -r confirm

if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    echo "Operation cancelled."
    exit 0
fi

# Get database connection info
if [ -z "$APP_NAME" ]; then
    echo "What is your Fly.io app name?"
    read -r APP_NAME
fi

if [ -z "$DB_NAME" ]; then
    echo "What is your Fly.io Postgres app name?"
    read -r DB_NAME
fi

echo "🔄 Detaching database from app..."
fly postgres detach -a "$APP_NAME" --database-app "$DB_NAME"

echo "🗑️ Destroying existing database..."
fly pg destroy "$DB_NAME" -y

echo "🏗️ Creating new database..."
fly pg create --name "$DB_NAME"

echo "🔗 Attaching new database to app..."
fly postgres attach --app "$APP_NAME" --database-app "$DB_NAME"

echo "🔄 Restarting app to use new database..."
fly apps restart "$APP_NAME"

echo "✅ Database has been recreated successfully!"
echo "Note: Your app should automatically create the schema on startup." 