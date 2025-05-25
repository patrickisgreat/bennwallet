ALTER TABLE transaction_categories DROP CONSTRAINT IF EXISTS transaction_categories_transaction_id_fkey;
ALTER TABLE transaction_categories DROP CONSTRAINT IF EXISTS transaction_categories_category_id_fkey;
ALTER TABLE transaction_categories
  ADD CONSTRAINT transaction_categories_transaction_id_fkey
    FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE;
ALTER TABLE transaction_categories
  ADD CONSTRAINT transaction_categories_category_id_fkey
    FOREIGN KEY (category_id) REFERENCES ynab_categories(id) ON DELETE CASCADE; 