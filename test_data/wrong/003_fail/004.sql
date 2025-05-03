-- migrate:up
CREATE VIEW active_users AS
SELECT u.id, u.username, d.name AS department_name
FROM users u
LEFT JOIN departments d ON u.department_id = d.id
WHERE u.username <> '';

-- migrate:down
DROP VIEW active_users;
