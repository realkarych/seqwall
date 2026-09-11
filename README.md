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
brew install realkarych/tap/seqwall
brew upgrade realkarych/tap/seqwall
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

After each migration, Seqwall captures schema metadata using both SQL-standard **`information_schema` views** and
**PostgreSQL-specific system catalogs and views**. The snapshots are compared using structured diffs.

The project-required and tested compatibility range is PostgreSQL 13–18. PostgreSQL 13 remains Seqwall's compatibility
floor even though it is outside upstream security maintenance; [upstream maintenance](https://www.postgresql.org/support/versioning/)
and Seqwall's tested range are separate policies. Changing either end of the range requires a ticket, compatibility
evidence, matching PostgreSQL integration and staircase matrices, and updated documentation.

Seqwall compares snapshots within one PostgreSQL major with unchanged connection and session settings. It does not
promise equality between snapshots serialized by different PostgreSQL majors. The `public` schema is selected by
default; repeat `--schema` to include more schemas. Only the properties captured below are compared.

| Object | Compared identity and properties |
| --- | --- |
| Tables/columns | Qualified table identity; ordered columns; column name, SQL/logical qualified type identity, type modifier/category, nullability, default, qualified collation, identity/generated state/expression, datetime precision, character length, numeric precision/scale |
| Views | Qualified identity and PostgreSQL view definition |
| Materialized views | Qualified identity, definition, and populated state |
| Indexes | Qualified identity and PostgreSQL index definition |
| Constraints | Qualified schema/table/name, full catalog-deparsed definition, type, deferrability, initial deferral, validation, inheritance, and enforcement where supported; CHECK/FK/PK/UNIQUE/exclusion plus native PostgreSQL 18 `NOT NULL` |
| Foreign keys | Qualified local/target relations, ordered local/target column lists, full definition, update/delete behavior; shared constraint state is represented by the constraint entry |
| Enums | Qualified identity and ordered labels |
| Triggers | Qualified owning table/name, full PostgreSQL definition, and enabled mode; user triggers including user constraint triggers, excluding internally generated triggers |
| Functions/procedures | Schema-qualified identity with argument types, routine kind, return type, and full definition; aggregates excluded |
| Sequences | Qualified identity, logical type, start/min/max/increment/cache/cycle, and nullable qualified column ownership/dependency kind; runtime counters excluded |
| Table privileges | Qualified table identity, grantee, privilege, and grantability visible through `role_table_grants` |

PostgreSQL 18's native `NOT NULL` metadata is captured as a real constraint, without regex or name rewriting. See the
[PostgreSQL 18 release notes](https://www.postgresql.org/docs/18/release-18.html) and
[`pg_constraint` catalog](https://www.postgresql.org/docs/18/catalog-pg-constraint.html).

Snapshot comparison does not establish universal database equivalence. Table persistence, row-level security,
partitioning, inheritance, relation options, and ownership are not comprehensively captured. Standalone domain,
composite, and range definitions and extension object definitions are not comprehensive. Non-table ACLs are not
comprehensive, and table grants are limited to privileges visible to the current role through `role_table_grants`.
Seqwall does not promise a consistent snapshot during concurrent DDL.

### `Staircase` tests captured schema consistency

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

This checks the captured schema metadata in both directions, including recovery from mid-chain downgrades.

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
The database schema and migration-runner history must both begin in the state immediately before the first listed
migration. Initialize the runner's metadata before starting Seqwall, without applying any listed migration. Any tables
or other objects created during that initialization become part of Seqwall's initial snapshot.

Seqwall selects non-directory files by `--migrations-extension` and sorts their paths lexicographically. Every upgrade
or downgrade command must advance exactly one migration. Callbacks inherit Seqwall's environment and working directory.

For dbmate 2.27.0, the following command creates its `schema_migrations` table without applying a migration:

```bash
export DATABASE_URL='postgres://postgres@localhost:5432/postgres?sslmode=disable'
dbmate --schema-file /dev/null dump
```

This requires `pg_dump` on `PATH`. The schema dump is discarded through `/dev/null`, while the bookkeeping table
remains in the database and becomes part of Seqwall's initial snapshot.

### Passing the current migration to your runner

Every upgrade and downgrade command receives the filename-safe current path through `SEQWALL_CURRENT_MIGRATION`,
even when the command contains no placeholder. Its value is the exact path discovered by Seqwall, without quoting or
normalization. Each command must apply or revert **exactly one migration**.
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

Seqwall focuses on reversibility and consistency of the captured schema metadata.

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
