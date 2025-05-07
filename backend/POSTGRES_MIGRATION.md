# PostgreSQL Migration Guide

This document outlines the steps taken to migrate the BennWallet application from SQLite to PostgreSQL, and how to maintain it going forward.

## Why PostgreSQL?

PostgreSQL offers several advantages over SQLite for a production application:

1. **Better concurrency**: PostgreSQL handles multiple connections much better than SQLite
2. **No database locking issues**: Eliminates the locking problems experienced with SQLite
3. **More scalable**: Can handle larger datasets and more concurrent users
4. **Better features**: Advanced features like JSON storage, full-text search, etc.
5. **Cloud-friendly**: Better suited for running in containerized environments like Fly.io

## Migration Steps Completed

We've made the following changes to support PostgreSQL:

1. **Added PostgreSQL driver** to `go.mod`
2. **Created PostgreSQL schema definition** in `database/postgres_schema.go`
3. **Added database type detection** to automatically use the right database
4. **Updated Dockerfile** to include PostgreSQL client libraries
5. **Created database initialization scripts** in the `scripts` directory
6. **Updated Fly.io configuration** to use PostgreSQL instead of SQLite volumes
7. **Consolidated schema management** to use a single approach for both databases
8. **Added compatibility layer** to handle both snake_case and camelCase column names

## How to Use PostgreSQL Locally

1. **Install PostgreSQL** if you haven't already.

2. **Create a new database**:

   ```bash
   createdb bennwallet
   ```

3. **Set environment variables**:

   ```bash
   export DB_HOST=localhost
   export DB_PORT=5432
   export DB_USER=postgres
   export DB_PASSWORD=your_password
   export DB_NAME=bennwallet
   ```

4. **Run the application**:
   ```
   go run .
   ```

## Deploying to Fly.io with PostgreSQL

1. **Create a PostgreSQL app on Fly.io**:
   ```
   fly postgres create --name bennwallet-db
   ```

2. **Attach the database to your app**:
   ```
   fly postgres attach --app bennwallet bennwallet-db
   ```

3. **Deploy the application**:
   ```
   fly deploy
   ```

## Resetting the Database

To reset the PostgreSQL database:

```
./scripts/reset_postgres_db.sh
```

Or set the environment variable when starting the application:

```
RESET_DB=true go run .
```

## Testing

The application still uses SQLite for tests by default. This is controlled by the `TEST_DB=1` environment variable.

## Schema Management

Instead of individual migration files, we now manage the schema in two main files:

- `database/postgres_schema.go` - PostgreSQL-specific schema
- `database/schema.go` - Database-agnostic schema handling

## Future Database Changes

When adding new tables or columns:

1. Add them to both `createPostgresSchema` and `createSQLiteSchema` functions in the respective schema files
2. Use the `AddColumn` helper function for adding columns to existing tables
3. Use the `CheckColumnExists` helper function to conditionally handle schema changes

## Troubleshooting

If you encounter issues:

1. **Check connection settings**: Verify that your PostgreSQL connection details are correct
2. **Database logs**: Check the PostgreSQL logs for any errors
3. **Reset the database**: Try resetting the database if schema issues persist
4. **Validate environment variables**: Ensure all required environment variables are set 