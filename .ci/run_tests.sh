#!/usr/bin/env bash
set -euo pipefail

PG_VERSION="${PG_VERSION:-15}"
PGPORT="${PGPORT:-5432}"
PGDATA_BASE="/tmp/pgdata-$PG_VERSION"
case_output=""
db_active=0

cleanup() {
  local status=$?
  local cleanup_status=0
  trap - EXIT INT TERM
  if [ -n "$case_output" ]; then
    cat "$case_output" || cleanup_status=$?
    rm -f "$case_output" || cleanup_status=$?
  fi
  if [ "$db_active" -eq 1 ]; then
    stop_db || cleanup_status=$?
  fi
  if [ "$status" -eq 0 ]; then
    status=$cleanup_status
  fi
  exit "$status"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

apt-get update -qq
if ! apt-cache show "postgresql-$PG_VERSION" >/dev/null 2>&1; then
  echo "deb http://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" \
    > /etc/apt/sources.list.d/pgdg.list
  apt-get install -yqq --no-install-recommends curl gnupg
  curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
  apt-get update -qq
fi
apt-get install -yqq \
  "postgresql-$PG_VERSION" \
  "postgresql-client-$PG_VERSION"

export DATABASE_URL="postgres://postgres@localhost:$PGPORT/postgres?sslmode=disable"

init_db() {
  rm -rf "$PGDATA_BASE"
  mkdir -p "$PGDATA_BASE"
  chown -R postgres:postgres "$PGDATA_BASE"
  su - postgres -c "/usr/lib/postgresql/$PG_VERSION/bin/initdb -D $PGDATA_BASE" \
    >/dev/null
  db_active=1
  su - postgres -c "/usr/lib/postgresql/$PG_VERSION/bin/pg_ctl \
    -D $PGDATA_BASE -o \"-p $PGPORT\" -w start" \
    >/dev/null
}

stop_db() {
  su - postgres -c "/usr/lib/postgresql/$PG_VERSION/bin/pg_ctl \
    -D $PGDATA_BASE -m fast -w stop" \
    >/dev/null
}

run_one() {
  local dir="$1"
  seqwall staircase \
    --migrations-path "$dir" \
    --upgrade 'MIGRATION_FILE="$SEQWALL_CURRENT_MIGRATION"; \
      MIGRATION_TMPDIR=$(mktemp -d); \
      cleanup_migration_dir() { rm -rf "$MIGRATION_TMPDIR"; }; \
      trap cleanup_migration_dir EXIT; \
      cp "$MIGRATION_FILE" "$MIGRATION_TMPDIR"; \
      DBMATE_MIGRATIONS_DIR="$MIGRATION_TMPDIR" \
      dbmate --no-dump-schema up' \
    --downgrade 'DBMATE_MIGRATIONS_DIR="'"$dir"'" \
      dbmate --no-dump-schema down' \
    --postgres-url "$DATABASE_URL" >"$case_output" 2>&1
}

initialize_runner() {
  dbmate --schema-file /dev/null dump >/dev/null
}

for tool in seqwall dbmate su rm mkdir chown mktemp cp cat grep \
  "/usr/lib/postgresql/$PG_VERSION/bin/initdb" \
  "/usr/lib/postgresql/$PG_VERSION/bin/pg_ctl" \
  "/usr/lib/postgresql/$PG_VERSION/bin/pg_isready"; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "❌ Missing required tool: $tool" >&2
    exit 1
  fi
done

shopt -s nullglob
valid_dirs=(test_data/valid/*/)
wrong_dirs=(test_data/wrong/*/)
if [ ! -d test_data/valid ] || [ "${#valid_dirs[@]}" -eq 0 ]; then
  echo "❌ Missing or empty fixture category: test_data/valid" >&2
  exit 1
fi
if [ ! -d test_data/wrong ] || [ "${#wrong_dirs[@]}" -eq 0 ]; then
  echo "❌ Missing or empty fixture category: test_data/wrong" >&2
  exit 1
fi

expected_failures=()
for d in "${wrong_dirs[@]}"; do
  if [ ! -f "$d.expected-failure" ]; then
    echo "❌ Missing expected-failure metadata: $d.expected-failure" >&2
    exit 1
  fi
  mapfile -t expected_lines < "$d.expected-failure"
  if ! grep -Iq . "$d.expected-failure" || [ "${#expected_lines[@]}" -ne 1 ] || \
    [[ "${expected_lines[0]}" != "snapshot after first down \"${d}"*".sql\": schema snapshots differ" ]]; then
    echo "❌ Invalid expected-failure metadata: $d.expected-failure" >&2
    exit 1
  fi
  expected_failures+=("${expected_lines[0]}")
done

run_case() {
  local dir="$1"
  local expected="$2"
  local status
  total=$((total+1))

  if [ ! -d "$dir" ]; then
    echo "❌ Missing fixture directory: $dir" >&2
    exit 1
  fi
  init_db
  "/usr/lib/postgresql/$PG_VERSION/bin/pg_isready" -d "$DATABASE_URL"
  initialize_runner
  "/usr/lib/postgresql/$PG_VERSION/bin/pg_isready" -d "$DATABASE_URL"
  if [ ! -d "$dir" ]; then
    echo "❌ Missing fixture directory: $dir" >&2
    exit 1
  fi

  case_output=$(mktemp)
  if run_one "$dir"; then
    status=0
  else
    status=$?
  fi
  cat "$case_output"
  if { [ -z "$expected" ] && [ "$status" -eq 0 ]; } || \
    { [ -n "$expected" ] && [ "$status" -ne 0 ] && grep -Fq -- "$expected" "$case_output"; }; then
    echo "✔  $dir"
    ok=$((ok+1))
  else
    echo "❌ $dir (Seqwall exit $status)"
    if [ -n "$expected" ]; then
      echo "  Expected a nonzero exit containing: $expected"
    else
      echo "  Expected success"
    fi
    failed_list+=("$dir")
    fail=$((fail+1))
  fi
  rm -f "$case_output"
  case_output=""
  stop_db
  db_active=0
}

total=0
ok=0
fail=0
failed_list=()

for d in "${valid_dirs[@]}"; do
  run_case "$d" ""
done
for i in "${!wrong_dirs[@]}"; do
  run_case "${wrong_dirs[$i]}" "${expected_failures[$i]}"
done

echo
echo "================ SUMMARY ================"
echo "  OK:    $ok / $total"
echo "  FAIL:  $fail"
if [ "$fail" -ne 0 ]; then
  printf '  Failed dirs:\n   %s\n' "${failed_list[@]}"
fi
echo "========================================="
[ "$fail" -eq 0 ]
