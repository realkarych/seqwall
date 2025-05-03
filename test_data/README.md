# Migrations for 2e2 tests

## Testcase types

- `valid/` folder contains subfolders. Each subfolder is a scenario to test. Each scenario is correct (no errors).
- `wrong/` folder contains subfolders. Each subfolder is a scenario to test. Each scenario is wrong (contain error(s), fails).

## Naming conventions

- In `valid/`, each test case folder **must be named with an ascending number** (`001/`, `002/`, `003/`, etc.) —
this ensures clear order and separation of correct migration scenarios.
- In `wrong/`, each test case folder **should be named either**:
  - after the **specific check or condition it is supposed to fail**, or
  - semantically describe the **kind of error or invalid behavior** being tested
  (e.g. `missing_down`, `wrong_column_type`, `non_reversible`).
