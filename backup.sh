#!/bin/bash
set -e

# Configuration
BACKUP_DIR="/data/backups"
DB_FILE="/data/bennwallet.db"  # Adjust this path to your actual database location
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="${BACKUP_DIR}/${TIMESTAMP}_bennwallet.db"
MAX_BACKUPS=7  # Keep a week of backups

# Ensure backup directory exists
mkdir -p $BACKUP_DIR

# Create the backup using sqlite3 with write-ahead log mode enabled
echo "Creating backup of ${DB_FILE} to ${BACKUP_FILE}..."
sqlite3 $DB_FILE "PRAGMA wal_checkpoint(FULL);"
sqlite3 $DB_FILE ".backup '${BACKUP_FILE}'"

# Compress the backup
echo "Compressing backup..."
gzip -f $BACKUP_FILE

# Cleanup old backups
echo "Cleaning up old backups, keeping latest ${MAX_BACKUPS}..."
ls -tp $BACKUP_DIR/*_bennwallet.db.gz | grep -v '/$' | tail -n +$((MAX_BACKUPS+1)) | xargs -I {} rm -- {} || true

echo "Backup completed: ${BACKUP_FILE}.gz"