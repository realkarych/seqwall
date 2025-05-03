-- migrate:up
DROP VIEW IF EXISTS active_users;

ALTER TABLE users
  ALTER COLUMN username TYPE VARCHAR(50);

CREATE VIEW active_users AS
SELECT u.id, u.username, d.name AS department_name
FROM users u
LEFT JOIN departments d ON u.department_id = d.id
WHERE u.username <> '';

-- migrate:down
DROP VIEW IF EXISTS active_users;

ALTER TABLE users
  ALTER COLUMN username TYPE TEXT,
  ALTER COLUMN username SET NOT NULL;

CREATE VIEW active_users AS
SELECT u.id, u.username, d.name AS department_name
FROM users u
LEFT JOIN departments d ON u.department_id = d.id
WHERE u.username <> '';
