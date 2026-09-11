# Migrations for 2e2 tests

## Testcase types

- `valid/` folder contains subfolders. Each subfolder is a scenario to test. Each scenario is correct (no errors).
- `wrong/` folder contains subfolders. Each subfolder is a scenario to test.
  Each scenario must fail for its declared schema defect.

## Naming conventions

- In `valid/`, each test case folder **must be named with an ascending number** (`001/`, `002/`, `003/`, etc.) —
this ensures clear order and separation of correct migration scenarios.
- In `wrong/`, each test case folder **should be named either**:
  - after the **specific check or condition it is supposed to fail**, or
  - semantically describe the **kind of error or invalid behavior** being tested
  (e.g. `missing_down`, `wrong_column_type`, `non_reversible`).

## Expected failures

Every `wrong/` case must include an `.expected-failure` file containing one nonempty
line: a literal output fragment with the first-down snapshot stage, the migration
path, and the `schema snapshots differ` sentinel. For example:

```text
snapshot after first down "test_data/wrong/003_fail/003.sql": schema snapshots differ
```

A negative case passes only when Seqwall exits nonzero and its combined standard
output and standard error contain that exact fragment. A zero exit, a different
error, or missing or invalid metadata fails the suite. The runner prints captured
output and removes its temporary file after each case, including on early exits.

The current defects are:

- `003_fail`: migration `003.sql` loses the foreign key during its first downgrade.
- `006_fail`: migration `006.sql` loses the `NOT NULL` constraint during its first downgrade.
- `007_fail`: migration `007.sql` changes the timestamp type during its first downgrade.

Both `valid/` and `wrong/` must exist and contain at least one case directory.
Dependency checks, PostgreSQL initialization and readiness, dbmate initialization,
fixture discovery, and database shutdown must succeed independently of the
expected failure. They cannot satisfy a negative case.
