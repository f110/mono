# Postgres

We use [PostgreSQL](https://www.postgresql.org) to store data served on
pkg.go.dev.

For additional information on our architecture, see the
[design document](design.md).

## Local development database

1. Install PostgreSQL on your machine for local development.
   It should use the default Postgres port of 5432.

   If you use a Mac, the easiest way to do that is through installing
   https://postgresapp.com.

   Another option is to use `docker`. The following docker command will start a
   server locally, publish the server's default port to the corresponding local
   machine port, and set a password for the default database user (named
   `postgres`).

   ```
   docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=pick_a_secret -e LANG=C postgres
   ```

   (NOTE: If you have already installed postgres on a workstation using `sudo apt-get install postgres`, you may have a server already running, and the above
   docker command will fail because it can't bind the port. At that point you can
   set `GO_DISCOVERY_DATABASE_TEST_`XXX environment variables to use your installed
   server, or stop the server using `pg_ctl stop` and use docker. The following
   assumes docker.)

   You must also install a postgres client (for example `psql`).

   At this point you should have a Postgres server running on your local machine
   at port 5432.

2. Set the following environment variables:

   - `GO_DISCOVERY_DATABASE_USER` (default: postgres)
   - `GO_DISCOVERY_DATABASE_PASSWORD` (default: '')
   - `GO_DISCOVERY_DATABASE_HOST` (default: localhost)
   - `GO_DISCOVERY_DATABASE_NAME` (default: discovery-db)

   See `internal/config/config.go` for details regarding construction of the
   database connection string.

3. Once you have Postgres installed, you should create the `discovery-db` database
   by running `devtools/create_local_db.sh`.

   Then apply migrations, as described in 'Migrations' below. You will need to do
   this each time a new migration is added, to keep your local schema up to date.

## Setting up for tests

Tests require a Postgres instance. If you followed step 1 in "Local development
database" above, then you have one.

Tests use the following environment variables:

- `GO_DISCOVERY_DATABASE_TEST_USER` (default: postgres)
- `GO_DISCOVERY_DATABASE_TEST_PASSWORD` (default: '')
- `GO_DISCOVERY_DATABASE_TEST_HOST` (default: localhost)
- `GO_DISCOVERY_DATABASE_TEST_PORT` (default: 5432)

If you followed the instructions in step 1 of "Local development database", then
you only need to set the password variable.

You don't need to create a database for testing; the tests will automatically
create a database for each package, with the name `discovery_{pkg}_test`. For
example, for internal/worker, tests run on the `discovery_worker_test` database.

If you ever run into issues with your test databases and need to reset them, you
can run `devtools/drop_test_dbs.sh`.

Run `./all.bash` to verify your setup.

## Migrations

Migrations are managed using
[github.com/golang-migrate/migrate](https://github.com/golang-migrate/migrate),
with the [CLI tool](https://github.com/golang-migrate/migrate/tree/master/cli).

If this is your first time using golang-migrate, check out the
[Getting Started guide](https://github.com/golang-migrate/migrate/blob/master/GETTING_STARTED.md).

To install the golang-migrate CLI, follow the instructions in the
[migrate CLI README](https://github.com/golang-migrate/migrate/blob/master/cmd/migrate/README.md).

### Creating a migration

To create a new migration:

```
devtools/create_migration.sh <title>
```

This creates two empty files in `/migrations`:

```
{version}_{title}.up.sql
{version}_{title}.down.sql
```

The two migration files are used to migrate "up" to the specified version from
the previous version, and to migrate "down" to the previous version. See
[golang-migrate/migrate/MIGRATIONS.md](https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md)
for details.

### Applying migrations for local development

Use the `migrate` CLI:

```
devtools/migrate_db.sh [up|down|force|version] {#}
```

If you are migrating for the first time, choose the "up" command.

For additional details, see
[golang-migrate/migrate/GETTING_STARTED.md#run-migrations](https://github.com/golang-migrate/migrate/blob/master/GETTING_STARTED.md#run-migrations).
