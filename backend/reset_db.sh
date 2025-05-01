#!/bin/bash

# Stop any running server
pkill -f "go run main.go" || echo "No server process found"

# Delete the database file
rm -f database.db
echo "Database file deleted"

# Delete any WAL or SHM files
rm -f database.db-shm
rm -f database.db-wal
echo "WAL and SHM files deleted"

# Start the server again to recreate the database
echo "Restarting server..."
go run main.go 