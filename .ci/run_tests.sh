#!/usr/bin/env bash
set -euo pipefail

PG_VERSION="${PG_VERSION:-15}"
PGPORT="${PGPORT:-5432}"
PGDATA="/tmp/pgdata-$PG_VERSION"

apt-get update -qq
if ! apt-cache show "postgresql-$PG_VERSION" >/dev/null 2>&1; then
  echo "deb http://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" > /etc/apt/sources.list.d/pgdg.list
  apt-get install -yqq --no-install-recommends curl gnupg
  curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
  apt-get update -qq
fi
apt-get install -yqq "postgresql-$PG_VERSION" "postgresql-client-$PG_VERSION"

mkdir -p "$PGDATA" && chown -R postgres:postgres "$PGDATA"
su - postgres -c "/usr/lib/postgresql/$PG_VERSION/bin/initdb -D $PGDATA" >/dev/null
su - postgres -c "/usr/lib/postgresql/$PG_VERSION/bin/pg_ctl -D $PGDATA -o \"-p $PGPORT\" -w start" >/dev/null

export DATABASE_URL="postgres://postgres@localhost:$PGPORT/postgres?sslmode=disable"

total=0; ok=0; fail=0
failed_list=()

run_one() {
  dir="$1"
  seqwall staircase \
      --migrations-path "$dir" \
      --upgrade 'MIGRATION_FILE="{current_migration}"; TMPDIR=$(mktemp -d); cp "$MIGRATION_FILE" "$TMPDIR"; DBMATE_MIGRATIONS_DIR="$TMPDIR" dbmate up; rm -rf "$TMPDIR"' \
      --downgrade "DBMATE_MIGRATIONS_DIR=\"$dir\" dbmate down" \
      --postgres-url "$DATABASE_URL"
}

for d in test_data/valid/*/ ; do
  [ -d "$d" ] || continue
  total=$((total+1))
  if run_one "$d"; then
    echo "✔  $d"
    ok=$((ok+1))
  else
    echo "❌ $d (expected success)"
    failed_list+=("$d")
    fail=$((fail+1))
  fi
done

for d in test_data/wrong/*/ ; do
  [ -d "$d" ] || continue
  total=$((total+1))
  if run_one "$d"; then
    echo "❌ $d (should fail)"
    failed_list+=("$d")
    fail=$((fail+1))
  else
    echo "✔  $d"
    ok=$((ok+1))
  fi
done

echo
echo "================ SUMMARY ================"
echo "  OK:    $ok / $total"
echo "  FAIL:  $fail"
if [ "$fail" -ne 0 ]; then
  printf '  Failed dirs:\n   %s\n' "${failed_list[@]}"
fi
echo "========================================="
[ "$fail" -eq 0 ] || exit 1
