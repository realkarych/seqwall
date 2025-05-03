-- migrate:up
CREATE TABLE departments (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  CONSTRAINT chk_department_name CHECK (char_length(name) > 0)
);

CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  username TEXT NOT NULL,
  department_id INTEGER,
  updated_at TIMESTAMP NOT NULL DEFAULT now(),
  CONSTRAINT fk_department FOREIGN KEY (department_id) REFERENCES departments(id)
);

-- migrate:down
DROP TABLE users;
DROP TABLE departments;
