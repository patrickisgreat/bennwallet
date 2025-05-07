# Database Migrations

## New Approach

We have moved away from individual migration files to a more consolidated approach. Instead of having separate files for each schema change, we now have a single schema definition for each database type (PostgreSQL and SQLite).

The old migration files in this directory are kept for historical reference but are no longer used by the application.

## How It Works Now

1. Database schema is defined in two main files:
   - `database/postgres_schema.go` for PostgreSQL
   - `database/schema.go` for both database types with common utilities

2. When the application starts, it detects which database type is being used based on environment variables:
   - If `TEST_DB=1` is set, it uses SQLite for tests
   - If `DATABASE_URL` or `DB_HOST`/`DB_PORT` are set, it uses PostgreSQL
   - Otherwise, it defaults to SQLite for backward compatibility

3. The `CreateSchema` function in `database/schema.go` creates all necessary tables based on the detected database type.

4. The application supports both PostgreSQL's snake_case and SQLite's camelCase column naming conventions for backward compatibility.

## PostgreSQL Setup

### Local Development

To set up PostgreSQL locally:

1. Install PostgreSQL if you haven't already
2. Create a new database: `createdb bennwallet`
3. Set environment variables:

   ```bash
   export DB_HOST=localhost
   export DB_PORT=5432
   export DB_USER=postgres
   export DB_PASSWORD=your_password
   export DB_NAME=bennwallet
   ```

### Fly.io Deployment

To set up PostgreSQL on Fly.io:

1. Create a PostgreSQL app:

   ```bash
   fly postgres create --name bennwallet-db
   ```

2. Attach the database to your app:

   ```bash
   fly postgres attach --app bennwallet bennwallet-db
   ```

3. This will automatically set the `DATABASE_URL` environment variable.

## Data Migration

If you need to migrate data from an existing SQLite database to PostgreSQL, you'll need to:

1. Export your SQLite data:

   ```bash
   sqlite3 your-sqlite-db.db .dump > dump.sql
   ```

2. Edit the dump file to convert SQLite syntax to PostgreSQL (change data types, boolean values, etc.)

3. Import into PostgreSQL:

   ```bash
   psql -h hostname -U username -d dbname < dump.sql
   ```

## Why This Change?

The new approach simplifies schema management by:

1. Defining the complete schema in one place instead of spread across multiple migration files
2. Making it easier to add new tables or columns
3. Supporting both PostgreSQL and SQLite from a single codebase
4. Eliminating database locking issues with PostgreSQL's better concurrency handling
5. Providing better performance for production use

## Running Migrations

Migrations will run automatically when the application starts up, as they're integrated into the database initialization process.

If you want to run migrations manually, you can use the provided utility:

```bash
cd backend
go run cmd/migrate/main.go
```

## Transaction Date Migration

The most recent migration adds a `transaction_date` column to the `transactions` table to fix the issue where transaction dates weren't being preserved separately from entry dates.

### Technical Details

The migration:

1. Adds a `transaction_date` column to the `transactions` table
2. Initially populates this column with the existing `date` values
3. Updates the backend code to properly store and retrieve both dates

After this migration, transactions will maintain separate values for:

- `date`: When the transaction was entered into the system
- `transaction_date`: When the transaction actually occurred

## Database Reset & Test Data Seeding

The application includes functionality to automatically reset and seed the database with test data in development and PR deployment environments. This is particularly useful for GitHub Actions deployments.

## How it Works

1. For development and PR branch deployments, the database will be automatically reset and populated with test data on each deployment.
2. This behavior is controlled by environment variables:
   - `RESET_DB=true`: Forcibly resets and repopulates the database
   - `APP_ENV=development`: Marks the environment as development
   - `PR_DEPLOYMENT=true`: Marks the environment as a PR deployment

## Important Notes

- This functionality is deliberately disabled in production environments (when `APP_ENV=production` or `NODE_ENV=production`).
- The system has safeguards to prevent accidental data loss in production.
- Test data includes sample users, transactions, categories, and permissions.

## Manually Reset Database

To manually reset the database locally, use the included `reset_dev_db.sh` script:

```bash
./reset_dev_db.sh
```

This script:

1. Stops any running server
2. Removes the database file
3. Starts the server with `RESET_DB=true`
4. Performs database integrity checks
5. Shuts down the server when complete

## Test Data Contents

The test data includes:

- Default users (admin, sarah, patrick)
- Sample transactions
- Sample categories
- Sample permissions

## Adding to Test Data

To modify the test data that gets seeded, edit the `SeedTestData` function in `backend/migrations/seed_test_data.go`.
