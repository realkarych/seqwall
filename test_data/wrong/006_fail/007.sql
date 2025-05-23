-- migrate:up
ALTER TABLE users
  ALTER COLUMN updated_at
    TYPE timestamptz
    USING updated_at AT TIME ZONE 'UTC';

-- migrate:down
ALTER TABLE users
  ALTER COLUMN updated_at
    TYPE timestamp without time zone
    USING (updated_at AT TIME ZONE 'UTC'),
  ALTER COLUMN updated_at SET NOT NULL;
