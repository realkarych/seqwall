-- migrate:up
ALTER TABLE users
  ALTER COLUMN updated_at
    TYPE timestamp
    USING updated_at AT TIME ZONE 'UTC';

-- migrate:down
ALTER TABLE users
  ALTER COLUMN updated_at
    TYPE timestamptz;
