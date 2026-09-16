// Command migrate applies database migrations to the LevelMix Turso database.
//
// Usage:
//
//	migrate                 # same as "up"
//	migrate up              # apply all pending migrations
//	migrate up-by-one       # apply the next pending migration only
//	migrate status          # show applied/pending (replaces the old --dry-run)
//	migrate version         # show current DB version
//	migrate down            # roll back the most recent migration
//	migrate create <name> sql
//	migrate -prod up        # required when the target is not a dev database
//
// Credentials come from TURSO_DB_URL and TURSO_AUTH_TOKEN, loaded from .env.dev
// then .env if present, otherwise from the process environment. Flags must
// precede the subcommand.
package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// migrationsDir must match the directory prefix inside embedMigrations.
const migrationsDir = "migrations"

// devDBMarker identifies a non-production database. Any target whose URL does
// not contain it requires -prod. Keep in sync with the dev DB name.
const devDBMarker = "-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var envFile string
	var prod bool
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.StringVar(&envFile, "env", "", "path to env file (default .env.dev, then .env)")
	fs.BoolVar(&prod, "prod", false, "allow running against a non-dev database")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) == 0 {
		args = []string{"up"}
	}
	cmd, cmdArgs := args[0], args[1:]

	// Sequential (00002_x.sql) rather than goose's default timestamp names, to
	// stay consistent with the 00001_ baseline.
	goose.SetSequential(true)

	// "create" writes a new file to the on-disk migrations dir. It must not
	// run against the embedded FS, and it needs no database connection.
	if cmd == "create" {
		if len(cmdArgs) == 0 {
			return errors.New(`create requires a name, e.g. migrate create add_acx_checks sql`)
		}
		if len(cmdArgs) == 1 {
			cmdArgs = append(cmdArgs, "sql")
		}
		return goose.RunContext(ctx, "create", nil, migrationsDir, cmdArgs...)
	}

	// Load an env file from the working directory if present. Real environment
	// variables always win, so this is safe on the deploy host.
	if envFile != "" {
		// Overload, not Load: an exported prod TURSO_DB_URL in the shell would
		// otherwise silently win and we would migrate the wrong database.
		if err := godotenv.Overload(envFile); err != nil {
			return fmt.Errorf("load %s: %w", envFile, err)
		}
	} else {
		// .env.dev first so a bare `migrate` targets dev, not prod. A malformed
		// .env.dev is reported rather than falling through to a prod .env.
		err := godotenv.Load(".env.dev")
		if os.IsNotExist(err) {
			err = godotenv.Load(".env")
		}
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "migrate: no env file loaded (%v), using process environment\n", err)
		}
	}

	dsn, err := buildDSN()
	if err != nil {
		return err
	}

	host := dsn
	if i := strings.Index(host, "?"); i > 0 {
		host = host[:i] // never print the auth token
	}
	fmt.Fprintf(os.Stderr, "migrate: target %s\n", host)

	// The gate is symmetric on purpose. Requiring -prod for a prod target stops
	// an accidental prod migration; rejecting -prod for a dev target stops the
	// inverse mistake, where .env.dev quietly wins, dev gets migrated, and prod
	// is left untouched while the operator believes it is done.
	isDev := strings.Contains(host, devDBMarker)
	switch {
	case !isDev && !prod:
		return fmt.Errorf("refusing to run %q against %s without -prod", cmd, host)
	case isDev && prod:
		return fmt.Errorf("-prod passed but %s is a dev database; drop -prod", host)
	}

	db, err := sql.Open("libsql", dsn)
	if err != nil {
		return fmt.Errorf("open libsql: %w", err)
	}
	defer db.Close()

	// Turso is a remote HTTP-backed database; a single connection keeps
	// migration ordering deterministic and avoids surprise concurrency.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("turso"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := goose.RunContext(ctx, cmd, db, migrationsDir, cmdArgs...); err != nil {
		return fmt.Errorf("goose %s: %w", cmd, err)
	}
	return nil
}

func buildDSN() (string, error) {
	url := os.Getenv("TURSO_DB_URL")
	if url == "" {
		return "", errors.New("TURSO_DB_URL is not set")
	}
	token := os.Getenv("TURSO_AUTH_TOKEN")
	if token == "" {
		return "", errors.New("TURSO_AUTH_TOKEN is not set")
	}
	return url + "?authToken=" + token, nil
}
