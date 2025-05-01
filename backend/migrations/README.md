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