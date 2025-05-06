# Database Migrations

This package handles database schema migrations for the Benn Wallet application.

## Running Migrations

Migrations will run automatically when the application starts up, as they're integrated into the database initialization process.

If you want to run migrations manually, you can use the provided utility:

```
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

# Database Reset & Test Data Seeding

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
