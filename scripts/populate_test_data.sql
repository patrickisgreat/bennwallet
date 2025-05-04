-- First, check if optional and userId columns exist in transactions table
PRAGMA table_info(transactions);

-- Add optional column if it doesn't exist
ALTER TABLE transactions ADD COLUMN optional BOOLEAN DEFAULT 0;

-- Add userId column if it doesn't exist
ALTER TABLE transactions ADD COLUMN userId TEXT;

-- Populate test users if not exists
INSERT OR IGNORE INTO users (username, name) 
VALUES 
  ('sarah.elizabeth.wallis@gmail.com', 'Sarah'),
  ('patrick', 'Patrick');

-- Clear existing test transactions if needed
DELETE FROM transactions WHERE description LIKE '%Test transaction%';

-- Get today's date for testing
-- Use a date string that matches the format in the logs (2025-05-04)
-- Today's date is used to ensure the filters work with current date ranges
-- Create test transactions with various combinations of payTo and enteredBy
-- Transactions entered by Sarah for Patrick to pay
INSERT INTO transactions (id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional, userId)
VALUES 
  ('test1', 100.00, 'Test transaction 1 (Sarah entered for Patrick)', '2025-05-04', '2025-05-04', 'Food', 'Patrick', 0, NULL, 'Sarah', 0, 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2'),
  ('test2', 200.00, 'Test transaction 2 (Sarah entered for Patrick)', '2025-05-04', '2025-05-04', 'Housing', 'Patrick', 0, NULL, 'Sarah', 0, 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2'),
  ('test3', 150.00, 'Test transaction 3 (Sarah entered for Patrick)', '2025-05-04', '2025-05-04', 'Entertainment', 'Patrick', 1, '2025-05-04', 'Sarah', 0, 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2');

-- Transactions entered by Patrick for Sarah to pay
INSERT INTO transactions (id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional, userId)
VALUES 
  ('test4', 120.00, 'Test transaction 4 (Patrick entered for Sarah)', '2025-05-04', '2025-05-04', 'Food', 'Sarah', 0, NULL, 'Patrick Bennett', 0, 'admin-user-1'),
  ('test5', 250.00, 'Test transaction 5 (Patrick entered for Sarah)', '2025-05-04', '2025-05-04', 'Utilities', 'Sarah', 0, NULL, 'Patrick Bennett', 0, 'admin-user-1'),
  ('test6', 180.00, 'Test transaction 6 (Patrick entered for Sarah)', '2025-05-04', '2025-05-04', 'Transportation', 'Sarah', 1, '2025-05-04', 'Patrick Bennett', 0, 'admin-user-1');

-- Additional transactions with variations in payTo field
INSERT INTO transactions (id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional, userId)
VALUES 
  ('test7', 90.00, 'Test transaction 7 (payTo Sarah Smith)', '2025-05-04', '2025-05-04', 'Food', 'Sarah Smith', 0, NULL, 'Patrick Bennett', 0, 'admin-user-1'),
  ('test8', 75.00, 'Test transaction 8 (payTo saraH - lowercase test)', '2025-05-04', '2025-05-04', 'Misc', 'saraH', 0, NULL, 'Patrick Bennett', 0, 'admin-user-1'),
  ('test9', 110.00, 'Test transaction 9 (payTo To Sarah From Patrick)', '2025-05-04', '2025-05-04', 'Gifts', 'To Sarah From Patrick', 0, NULL, 'Patrick Bennett', 0, 'admin-user-1');

-- Optional transactions for testing optional filter
INSERT INTO transactions (id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional, userId)
VALUES 
  ('test10', 50.00, 'Test transaction 10 (optional)', '2025-05-04', '2025-05-04', 'Entertainment', 'Sarah', 0, NULL, 'Patrick Bennett', 1, 'admin-user-1'),
  ('test11', 65.00, 'Test transaction 11 (optional)', '2025-05-04', '2025-05-04', 'Food', 'Patrick', 0, NULL, 'Sarah', 1, 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2');

-- Variations in enteredBy field
INSERT INTO transactions (id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional, userId)
VALUES 
  ('test12', 85.00, 'Test transaction 12 (enteredBy Sarah Williams)', '2025-05-04', '2025-05-04', 'Groceries', 'Patrick', 0, NULL, 'Sarah Williams', 0, 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2'),
  ('test13', 95.00, 'Test transaction 13 (enteredBy SARAH - uppercase test)', '2025-05-04', '2025-05-04', 'Household', 'Patrick', 0, NULL, 'SARAH', 0, 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2'),
  ('test14', 105.00, 'Test transaction 14 (enteredBy Entered by Sarah)', '2025-05-04', '2025-05-04', 'Clothing', 'Patrick', 0, NULL, 'Entered by Sarah', 0, 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2'); 