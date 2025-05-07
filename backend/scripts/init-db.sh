#!/bin/bash
set -e

# This script initializes the PostgreSQL database for the bennwallet application
# It can be used in Docker to initialize the database on first run

echo "Initializing database..."

# If DATABASE_URL is set, use it directly
if [ -n "$DATABASE_URL" ]; then
  echo "Using DATABASE_URL for connection"
  # Test the connection
  PGPASSWORD=$DB_PASSWORD psql "$DATABASE_URL" -c "SELECT 1" > /dev/null 2>&1 || {
    echo "Cannot connect to PostgreSQL using DATABASE_URL. Please check your connection settings."
    exit 1
  }
else
  # Otherwise use individual components
  echo "Using individual connection parameters"
  
  # Make sure required variables are set
  if [ -z "$DB_HOST" ] || [ -z "$DB_PORT" ] || [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
    echo "ERROR: Database connection parameters not set. Please set DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME"
    exit 1
  fi
  
  # Test the connection
  PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" > /dev/null 2>&1 || {
    echo "Cannot connect to PostgreSQL. Please check your connection settings."
    
    # Try to create the database if it doesn't exist
    if PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -c "CREATE DATABASE $DB_NAME;" > /dev/null 2>&1; then
      echo "Created database $DB_NAME"
    else
      echo "Failed to create database $DB_NAME"
      exit 1
    fi
  }
fi

echo "Database connection successful."

# The actual database initialization is handled by the application at startup
# This script is just to ensure the database exists and is accessible

exit 0 