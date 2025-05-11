-- Drop the existing ynab_config table
DROP TABLE IF EXISTS ynab_config CASCADE;

-- Create it with the correct column names
CREATE TABLE IF NOT EXISTS ynab_config (
    id SERIAL PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    encrypted_api_token TEXT,
    encrypted_budget_id TEXT,
    encrypted_account_id TEXT,
    last_sync_time TIMESTAMP,
    sync_frequency INTEGER DEFAULT 60,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
); 