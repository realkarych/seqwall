-- migrate:up
CREATE EXTENSION IF NOT EXISTS citext;

-- migrate:down
DROP EXTENSION IF EXISTS citext;
