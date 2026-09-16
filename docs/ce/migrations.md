# LevelMix - Database Migrations

Schema changes are version-controlled with [goose](https://github.com/pressly/goose)
and applied automatically on deploy. Nothing is applied by hand in the Turso UI.

Migration files live in `cmd/migrate/migrations/` and are compiled **into** the
migrate binary with `//go:embed`. That means the deployed binary carries its own
migrations, and it also means the binary must be rebuilt whenever a migration
file changes — `migrate.sh` always rebuilds, so this is automatic.

## Dev and prod are kept apart by a filename

There is one rule, and everything else follows from it:

> **`.env.dev` is preferred over `.env`.** If `.env.dev` exists, it wins.

This applies to the server, worker, cleanup job **and** the migrate tool. The
effect is that the same commands do the right thing in each place without any
flags or profiles:

| Where | Files present | Result |
|---|---|---|
| Your laptop | `.env.dev` and `.env` | **dev database** — app and migrations |
| Hetzner, app containers | neither (excluded from the image) | prod, from the Docker environment |
| Hetzner, host (`migrate.sh`) | `.env` only | prod, and gated behind `-prod` |

So a bare `go run` or `./migrate.sh` on your machine can never touch production.
To get a dev database, copy `.env.example` to `.env.dev` and point it at a Turso
database whose **name contains `-dev`** (see the next section for why).

## The `-prod` flag

Any target whose URL does not contain `-dev` is treated as production and
requires the `-prod` flag. The check is symmetric — passing `-prod` at a dev
database is *also* an error:

| Target | Flag | Result |
|---|---|---|
| `…-dev…` | none | runs |
| `…-dev…` | `-prod` | **refused** — "drop `-prod`" |
| anything else | none | **refused** — "without `-prod`" |
| anything else | `-prod` | runs |

The second row is the one that saves you. Without it, running `-prod` from your
laptop would silently migrate *dev* (because `.env.dev` wins), leave production
untouched, and report success.

Which means: **from your laptop, targeting prod needs `-env .env -prod`**, because
you have to override the `.env.dev` preference as well as pass the flag. On the
server there is no `.env.dev`, so plain `-prod` is correct there.

## Everyday workflow

**1. Create the migration file.** `create` writes to `./migrations`, so run it
from `cmd/migrate`:

```bash
cd cmd/migrate
go run . create add_acx_checks sql     # -> migrations/00002_add_acx_checks.sql
```

**2. Write the SQL.** The generated file has `Up` and `Down` sections:

```sql
-- +goose Up
ALTER TABLE audio_files ADD COLUMN acx_checked BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE audio_files DROP COLUMN acx_checked;
```

**3. Apply it to dev and check it.** From the repo root:

```bash
./migrate.sh status     # what's applied, what's pending
./migrate.sh up         # apply everything pending
./migrate.sh down       # roll back the most recent migration
```

**4. Commit the migration file** along with whatever code depends on it.

**5. Deploy.** `./deploy.sh` runs `./migrate.sh -prod` at Step 4, between building
the images and restarting services. If a migration fails the deploy aborts
*before* the restart, so the old code keeps running against the old schema.

## Commands

| Command | Does |
|---|---|
| `./migrate.sh` | same as `up` |
| `./migrate.sh up` | apply all pending migrations |
| `./migrate.sh up-by-one` | apply only the next pending migration |
| `./migrate.sh status` | list applied and pending |
| `./migrate.sh version` | current database version |
| `./migrate.sh down` | roll back the most recent migration |
| `./migrate.sh create <name> sql` | new migration file (run from `cmd/migrate`) |

Add `-prod` for a production target. Add `-env <path>` to force a specific env
file. **Flags must come before the subcommand** — Go's flag parser stops at the
first non-flag argument, so `./migrate.sh up -prod` silently ignores the flag.

## Things that will bite you

**SQLite has no `ADD COLUMN IF NOT EXISTS`.** `CREATE TABLE IF NOT EXISTS` and
`CREATE INDEX IF NOT EXISTS` are fine; `ALTER TABLE … ADD COLUMN IF NOT EXISTS`
is a syntax error. Write the `ALTER` plainly and let the migration run once.

**`status` is not read-only against a fresh database.** The first time goose
connects it creates its own `goose_db_version` table and inserts a version-0 row.
Harmless, but it is a write.

**Each migration runs in a transaction.** If a statement can't run inside one,
add `-- +goose NO TRANSACTION` at the top of that file.

**Don't copy `00001`'s style.** The baseline uses `IF NOT EXISTS` on every
statement so it can no-op against the already-populated production database.
That is correct for a baseline and wrong everywhere else — in a normal migration
it would hide a failure instead of surfacing it.

**Number sequentially.** `create` is configured for sequential numbering
(`00002_`, `00003_`, …) rather than timestamps. Keep it that way so file order
matches apply order.

## Requirements on the deploy host

`migrate.sh` compiles the migrate binary with `go build`, so the host needs a
**Go toolchain**. Everything else in `deploy.sh` compiles inside Docker, so this
is the only host-side Go dependency. If `go` is missing, Step 4 fails and the
deploy aborts before restarting anything.

## How it fits together

| Piece | Role |
|---|---|
| `cmd/migrate/main.go` | goose wrapper: loads env, opens Turso, enforces the `-prod` gate |
| `cmd/migrate/migrations/*.sql` | the migrations, embedded into the binary |
| `migrate.sh` | builds `./bin/migrate` and forwards arguments to it |
| `deploy.sh` (Step 4) | runs `./migrate.sh -prod` before restarting services |
| `goose_db_version` | goose's own table in each database, tracking applied versions |
