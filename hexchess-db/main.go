package main

import (
	"bufio"
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

//go:embed migrations/*
var migrationsFS embed.FS

func initEnv() {
	file, err := os.Open(".env")
	if err != nil {
		slog.Warn("did not load env file", "err", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		index := strings.Index(line, "=")
		if index < 0 {
			log.Fatalf("invalid line in .env file: %s", line)
		}
		key, value := line[:index], line[index+1:]
		if err := os.Setenv(key, value); err != nil {
			slog.Warn("error setting env var", "err", err)
		}
	}
}

var migrFlag = flag.String("migration", "", "the migration type to run, one of: up, down, goto, force")
var versionFlag = flag.String("version", "", "the migration version to run to")

func main() {
	flag.Parse()
	initEnv()

	dbURL := os.Getenv("DB_URL")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("failed to migrate driver: %v", err)
	}
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		log.Fatalf("failed to make iofs source: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		log.Fatalf("failed to migrate instance: %v", err)
	}
	migrType := *migrFlag
	if err := runMigration(m, migrType, *versionFlag); err != nil {
		log.Fatalf("failed to run migration: %v", err)
	}
	version, dirty, err := m.Version()
	if err != nil {
		log.Fatalf("failed to get migration version: %v", err)
	}
	log.Printf("database migrations applied successfully, version=%d, dirty=%v", version, dirty)
}

func runMigration(m *migrate.Migrate, migrType string, versionFlag string) error {
	switch strings.ToUpper(migrType) {

	case "UP":
		if err := m.Up(); err != nil {
			return fmt.Errorf("failed to migrate UP: %w", err)
		}
	case "DOWN":
		if err := m.Down(); err != nil {
			return fmt.Errorf("failed to migrate DOWN: %w", err)
		}
	case "FORCE":
		version, err := strconv.Atoi(versionFlag)
		if err != nil {
			return fmt.Errorf("failed to parse FORCE version %q: %w", versionFlag, err)
		}
		if err := m.Force(version); err != nil {
			return fmt.Errorf("failed to migrate FORCE %d: %w", version, err)
		}
	case "GOTO":
		steps, err := strconv.Atoi(versionFlag)
		if err != nil {
			return fmt.Errorf("failed to parse GOTO version %q: %w", versionFlag, err)
		}
		if err := m.Steps(steps); err != nil {
			return fmt.Errorf("failed to migrate GOTO %d: %w", steps, err)
		}
	default:
		return fmt.Errorf("invalid migration type: %s", migrType)
	}
	return nil
}
