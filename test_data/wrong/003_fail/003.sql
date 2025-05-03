-- migrate:up
ALTER TABLE users
  ADD COLUMN mood mood NOT NULL DEFAULT 'ok';

-- migrate:down
ALTER TABLE users
  DROP CONSTRAINT IF EXISTS fk_department;
ALTER TABLE users
  DROP COLUMN mood;
