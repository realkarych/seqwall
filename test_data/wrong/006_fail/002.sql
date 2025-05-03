-- migrate:up
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_type WHERE typname = 'mood'
  ) THEN
    CREATE TYPE mood AS ENUM ('sad', 'ok', 'happy');
  END IF;
END $$;

-- migrate:down
DROP TYPE IF EXISTS mood;
