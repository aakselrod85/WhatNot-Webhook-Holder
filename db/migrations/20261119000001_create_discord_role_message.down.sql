-- Deliberately empty: the table existed in production before this stand-in migration was written and holds rows
-- that code outside this repository created, so rolling back must not drop it.
SELECT 1;
