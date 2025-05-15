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
sudo apt upgrade  seqwall         # later updates
```

### Other distros / Windows

Download the pre‑built archive from the **[Releases](https://github.com/realkarych/seqwall/releases)** page, unpack,
add the binary to your `PATH`.

> On Windows, you may need `Unblock-File .\seqwall.exe` before first run.

### Go install (Go ≥ 1.17)

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
      --postgres-url string           PostgreSQL URL (required or fallback: $DATABASE_URL environment variable)
      --migrations-path string        Path to migrations. Migrations must be in lexicographical order (required)
      --upgrade string                Shell command that applies next migration (required)
      --downgrade string              Shell command that reverts current migration (required)
      --migrations-extension string   Extension of migration files (default: .sql)
      --schema stringArray            Schemas to test (default [public])
      --test-snapshots                Compare schema snapshots. If false, only checks fact that migrations are applied
                                      / reverted with no errors (default true)
      --depth int                     Depth of staircase testing (0 = all)
      --help                          help for staircase
```

<hr>

## <p align=center>🧬 Methodology & Core Principles</p>

### Migrations are contracts

Each migration must be reversible and must not break the schema if applied, reverted, and reapplied.

### Snapshots reveal the truth

After each migration, Seqwall captures the schema using **`information_schema` views**,
adhering to the **<a href="https://www.iso.org/standard/76586.html">ISO/IEC 9075-11</a> SQL standard**.

This includes *tables*, *columns*, *constraints*, *indexes*, *views*,
*triggers*, *functions*, *enums*, *sequences*, and *foreign keys*.
The snapshots are then compared using structured diffs, allowing detection of even subtle schema differences or mismatches.

### `Staircase` testing guarantees *schema* consistency

We use a 3-phase strategy:

1. **`actualize`** — applying all migrations and captures *etalon* schema snapshot for each migration.

2. **`down → up → down`** — starting from the latest migration, step backwards:
   - downgrade one migration,
   - upgrade it again,
   - then downgrade once more (down step).
   - At each step, the schema is compared with previously captured
   *etalon* snapshots — both before and after — ensuring reversibility and no drift.

3. **`up → down → up`** — starting from the lower point reached in step 2 (after several rollbacks):
   - re-apply each migration one by one,
   - after each, immediately downgrade it,
   - then re-apply again (up step).

This ensures that the migration chain is robust in both directions, even when recovering from mid-chain downgrades.

### Standalone by design

Seqwall is a single-purpose CLI tool — it requires no server, no daemon, no embedded framework, and no special runtime.

You can run it locally or in CI/CD (recommended), with just your migrations and a database connection string.
No vendor lock-in, no config-files, no dependencies beyond PostgreSQL.

### Test migrations as they really run

Seqwall runs your actual migration scripts and commands — no wrapper DSLs, no abstractions, no mocks.

You bring your own migration runner (`dbmate`, `alembic`, `goose`, `sqlx`, `atlas`, etc.).
Seqwall just executes shell commands.

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
