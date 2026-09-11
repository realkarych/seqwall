<a href="https://github.com/realkarych/seqwall">
<p align="center" width="100%">
    <img width="50%" alt="seqwall logo" src="https://github.com/user-attachments/assets/4ff7fce5-4e74-44ff-a6af-bb50d39449a3">
</p>
</a>

<p align="center">
  <a href="https://github.com/realkarych/seqwall">Seqwall</a> is a tool for PostgreSQL migrations testing.<br>
  Ensure that every migration is reversible, idempotent, compatible with others in sequence, structurally sound and verifiable.
</p>

<!-- Badges -->
<p align="center">
  <a href="https://github.com/realkarych/seqwall/actions/workflows/ci.yml"><img alt="CI status" src="https://github.com/realkarych/seqwall/actions/workflows/ci.yml/badge.svg"></a>&nbsp;<!--
  --><a href="https://app.codecov.io/gh/realkarych/seqwall"><img alt="coverage" src="https://codecov.io/gh/realkarych/seqwall/branch/master/graph/badge.svg"></a>&nbsp;<!--
  --><a href="https://go.dev"><img alt="go version" src="https://img.shields.io/github/go-mod/go-version/realkarych/seqwall"></a>&nbsp;<!--
  --><a href="https://github.com/realkarych/seqwall/blob/master/LICENSE"><img alt="license MIT" src="https://img.shields.io/github/license/realkarych/seqwall"></a>&nbsp;<!--
  --><img alt="platforms" src="https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-blue">
</p>

<hr>

## <p align=center>📦 Installation</p>

### Docker images

**Package:** <https://github.com/realkarych/seqwall/pkgs/container/seqwall>.

```bash
docker run --rm --network=host \
  ghcr.io/realkarych/seqwall:latest staircase --help
```

### Homebrew (macOS & Linux)

```bash
brew tap realkarych/tap
brew install seqwall        # first install
brew upgrade seqwall        # later updates
```

### Debian / Ubuntu (APT)

```bash
# Import the GPG key
curl -fsSL https://realkarych.github.io/seqwall-apt/public.key \
  | sudo tee /etc/apt/trusted.gpg.d/seqwall.asc

# Add the repository
echo "deb [arch=$(dpkg --print-architecture)] \
  https://realkarych.github.io/seqwall-apt stable main" \
  | sudo tee /etc/apt/sources.list.d/seqwall.list

# Install / update
sudo apt update
sudo apt install seqwall          # first install
sudo apt upgrade seqwall          # later updates
```

### Other distros / Windows

Download the pre‑built archive from the **[Releases](https://github.com/realkarych/seqwall/releases)** page, unpack,
add the binary to your `PATH`.

> On Windows, you may need `Unblock-File .\seqwall.exe` before first run.

### Go install (Go ≥ 1.24)

```bash
go install github.com/realkarych/seqwall@latest
# make sure $GOBIN (default ~/go/bin) is on your PATH
```

<hr>

### ✅ Once installed, verify it works

```
❯ seqwall staircase --help
Launch staircase testing

Usage:
  seqwall staircase [flags]

Flags:
      --postgres-url string           PostgreSQL connection URL (defaults to DATABASE_URL)
      --migrations-path string        Directory containing lexicographically ordered migration files
      --upgrade string                Command that applies exactly one migration
      --downgrade string              Command that reverts exactly one migration
      --test-snapshots                Compare schema snapshots (default true)
      --schema stringArray            Schema to include in testing (repeatable) (default [public])
      --depth int                     Number of migrations to test (0 means all)
      --migrations-extension string   Migration filename extension (default ".sql")
  -h, --help                          help for staircase
```

<hr>

## <p align=center>🧬 Methodology & Core Principles</p>

### Migrations are contracts

Each migration must be reversible and must not break the schema if applied, reverted, and reapplied.

### Snapshots reveal the truth

After each migration, Seqwall captures schema metadata using **`information_schema` views**
and **PostgreSQL system catalogs**.

This includes *tables*, *columns*, *constraints*, *indexes*, *views*,
*triggers*, *functions*, *enums*, *sequences*, and *foreign keys*.
Constraint snapshots retain complete definitions and enforcement state. Foreign key snapshots retain ordered
local and referenced columns with qualified table identities.
Column snapshots record effective nullability on every supported PostgreSQL version. On PostgreSQL 18, constraint
snapshots also retain native `NOT NULL` names, definitions, validation state, enforcement, and inheritance behavior.
Trigger snapshots retain complete definitions and enabled state for user-defined triggers, including constraint
triggers. PostgreSQL-generated internal triggers are excluded, so custom changes to their firing mode are not compared.
Sequence snapshots retain their numeric type, start, minimum, maximum, increment, cycle and cache configuration,
plus qualified column ownership for explicit, serial and identity sequences. Runtime counters such as the current or
last value and `is_called` are data state and are excluded.
Column references to domain, composite, and range types retain their qualified type identity, and enum labels are
captured. Snapshot coverage is deliberately bounded: table persistence, row-level security policies, partition and
inheritance metadata, relation options and ownership, complete domain/composite/range definitions, extension object
definitions, non-table ACLs, and role-dependent table-grant visibility are not comprehensively captured.
The snapshots are compared using structured diffs. This comparison covers the captured metadata in the selected
schemas; it does not establish universal database equivalence or a transactionally consistent view during concurrent DDL.

### `Staircase` testing guarantees *schema* consistency

We use a 3-phase strategy:

1. **`actualize`** — applying all migrations and captures *etalon* schema snapshot for each migration.

2. **`down → up → down`** — starting from the latest migration, step backwards:
   - downgrade one migration,
   - upgrade it again,
   - then downgrade once more (down step).
   - At each step, the schema is compared with previously captured
   *etalon* snapshots — both before and after — ensuring reversibility and no drift.

3. **`re-actualize`** — starting from the lower point reached in step 2 (after several rollbacks):
   - re-apply each migration one by one
   - compare each re-applied migration with etalon

This ensures that the migration chain is robust in both directions, even when recovering from mid-chain downgrades.

<p align="center" width="100%">
    <img width="75%" alt="staircase" src="https://github.com/user-attachments/assets/b3fad935-a08b-483c-ada1-68586288f6b7">
</p>

### Standalone by design

Seqwall is a single-purpose CLI tool — it requires no server, no daemon, no embedded framework, and no special runtime.

You can run it locally or in CI/CD (recommended), with just your migrations and a database connection string.
No vendor lock-in, no config-files, no dependencies beyond PostgreSQL.

### Test migrations as they really run

Seqwall runs your actual migration scripts and commands — no wrapper DSLs, no abstractions, no mocks.

You bring your own migration runner (`dbmate`, `alembic`, `goose`, `sqlx`, `atlas`, etc.).
Seqwall just executes shell commands.

Seqwall captures the database state before the first migration and expects the first rollback to restore that state.
Initialize any tables or other objects that your migration runner needs before starting Seqwall.

For dbmate 2.27.0, the following command creates its `schema_migrations` table without applying a migration:

```bash
export DATABASE_URL='postgres://postgres@localhost:5432/postgres?sslmode=disable'
dbmate --schema-file /dev/null dump
```

This requires `pg_dump` on `PATH`. The schema dump is discarded through `/dev/null`, while the bookkeeping table
remains in the database and becomes part of Seqwall's initial snapshot.

### Passing the current migration to your runner

Every upgrade and downgrade command receives `SEQWALL_CURRENT_MIGRATION` in its environment,
even when the command contains no placeholder. Its value is the exact path discovered by Seqwall,
without quoting or normalization. Each command must apply or revert **exactly one migration**.
An unrestricted `up` that applies all pending migrations violates the staircase algorithm.

For a POSIX-compatible shell, pass the value as a double-quoted argument:

```sh
seqwall staircase --postgres-url "$DATABASE_URL" --migrations-path ./migrations \
  --upgrade './migrate-one up "$SEQWALL_CURRENT_MIGRATION"' \
  --downgrade './migrate-one down "$SEQWALL_CURRENT_MIGRATION"'
```

The outer single quotes defer expansion until Seqwall runs the command. Inside a shell script,
use `"$SEQWALL_CURRENT_MIGRATION"` in the same way. Repeat the quoted variable to pass the path twice.
Do not embed the value in source passed to `eval` or another `sh -c`.
Seqwall uses `$SHELL`, falling back to `sh` when unset or empty; a non-POSIX shell requires its own
safe variable syntax, or a helper that reads the environment directly.

On Windows, use a native helper that reads `SEQWALL_CURRENT_MIGRATION`, or invoke PowerShell scripts
without inserting the filename into the command string:

```text
--upgrade "powershell.exe -NoProfile -File .\apply-one.ps1"
--downgrade "powershell.exe -NoProfile -File .\revert-one.ps1"
```

The scripts read `$env:SEQWALL_CURRENT_MIGRATION` as a string; for example,
`Get-Content -LiteralPath $env:SEQWALL_CURRENT_MIGRATION` reads that exact file.
A native helper can pass the value to its runner using an argument list.
Expanding `%SEQWALL_CURRENT_MIGRATION%` in cmd is not a universal literal-data contract:
delayed expansion, `CALL`, nested parsing, and command construction can reinterpret the filename.

The legacy `{current_migration}` placeholder retains raw source substitution for filenames containing
only ASCII letters, digits, `_`, `-`, `.`, and `/`; Windows also permits `\` and `:`.
An empty value retains the previous empty substitution behavior. Every other filename is rejected
before the shell starts if the command contains a placeholder, including quoted, embedded, or repeated
placeholders. This deliberately restricts previously accepted templates with spaces or punctuation;
use the environment contract for those names. Legacy substitution preserves simple filename behavior
and does not promise literal arguments in arbitrary shell evaluation contexts.

### Limitations & Scope

Does this mean Seqwall is the only tool you need for testing migrations?

No — databases involve a spectrum of concerns, and a complete testing strategy should include:

- Load testing — to observe performance & regressions
- Lock behavior analysis — to catch deadlocks and blocking issues
- Data state testing — to ensure data survives or transforms as expected
- Static analysis — to catch anti-patterns or unsafe operations before runtime
- Integration tests — to validate application logic against migrated schemas
- ...

Seqwall focuses on **schema-level structural correctness** — nothing more, nothing less.

## <p align="center">🙏 Contribution</p>

### Found a bug?

- Please [open an issue](https://github.com/realkarych/seqwall/issues/new?template=bug.yml) with a clear description,
reproduction steps (if possible), and expected vs. actual behavior.

### Have a question?

- Please [open a discussion](https://github.com/realkarych/seqwall/discussions/categories/q-a) in QA section.
Or feel free to message me on Telegram: [`@karych`](https://t.me/karych).

### Want to suggest a feature?

- If you have a concrete and well-scoped idea — feel free to [open a feature request](https://github.com/realkarych/seqwall/issues/new?template=feature_request.yml).
- If the idea is more exploratory — start a
[discussion](https://github.com/realkarych/seqwall/discussions/categories/ideas) instead.

### Ready to contribute code?

- Look for issues marked with `help wanted` or `good first issue`. *In fact, you can pick any issue without Assignees* 😊️️️️️️.
- Fork the repo, create a branch, and open a pull request when ready (and tag `@realkarych` for review).

Your feedback and contributions are always welcome 💙.
