ALTER TABLE max_users
    ADD COLUMN IF NOT EXISTS first_name TEXT,
    ADD COLUMN IF NOT EXISTS last_name TEXT;

UPDATE max_users
SET first_name = NULLIF(TRIM(COALESCE(full_name, '')), ''),
    last_name = NULL
WHERE first_name IS NULL;

ALTER TABLE max_users
    DROP COLUMN IF EXISTS full_name;
