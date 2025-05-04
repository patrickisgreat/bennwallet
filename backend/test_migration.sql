-- Test script for verifying Sarah's migration
-- First, let's check the current users
SELECT id, name, username, status, isAdmin FROM users;

-- Let's simulate what happens when Sarah signs in with her Firebase UID
-- This would be done by the SyncFirebaseUser handler
UPDATE users 
SET id = 'SimulatedSarahUID', 
    username = 'sarah.elizabeth.wallis@gmail.com', 
    name = 'Sarah Elizabeth Wallis', 
    isAdmin = 1 
WHERE id = '1' AND name = 'Sarah';

-- Update transactions to use the new ID
UPDATE transactions 
SET userId = 'SimulatedSarahUID' 
WHERE userId = '1';

-- Verify the changes
SELECT id, name, username, status, isAdmin FROM users;

-- Check transactions for the user
SELECT id, amount, description, userId 
FROM transactions 
WHERE userId = 'SimulatedSarahUID' 
LIMIT 5;

-- Test the approved sharing group query
SELECT id, name 
FROM users 
WHERE name = 'Patrick' OR name = 'Sarah' OR 
      username = 'patrick' OR username = 'sarah' OR
      id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2' OR
      username = 'sarah.elizabeth.wallis@gmail.com';

-- After verification, revert the changes
-- UPDATE users 
-- SET id = '1',
--     username = 'sarah',
--     name = 'Sarah',
--     isAdmin = 0
-- WHERE id = 'SimulatedSarahUID';

-- UPDATE transactions 
-- SET userId = '1' 
-- WHERE userId = 'SimulatedSarahUID'; 