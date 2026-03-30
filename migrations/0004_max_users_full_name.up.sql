ALTER TABLE max_users
    ADD COLUMN IF NOT EXISTS full_name TEXT;

UPDATE max_users
SET full_name = NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', COALESCE(first_name, ''), COALESCE(last_name, ''))), '')
WHERE NULLIF(TRIM(COALESCE(full_name, '')), '') IS NULL;

ALTER TABLE max_users
    DROP COLUMN IF EXISTS first_name,
    DROP COLUMN IF EXISTS last_name;
