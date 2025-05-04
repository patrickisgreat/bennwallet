-- Reset the test changes
UPDATE users 
SET id = '1',
    username = 'sarah',
    name = 'Sarah',
    isAdmin = 0
WHERE id = 'SimulatedSarahUID';

UPDATE transactions 
SET userId = '1' 
WHERE userId = 'SimulatedSarahUID'; 