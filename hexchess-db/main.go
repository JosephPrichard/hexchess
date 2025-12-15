package main

import (
	"bufio"
	"database/sql"
	"embed"
	"errors"
	"flag"
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

var migrFlag = flag.String("migration", "UP", "the migration type to run, one of: up, down, or a version number")

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
	switch strings.ToUpper(migrType) {
	case "DOWN":
		log.Fatal("database migration down not supported")
	case "UP":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("failed to migrate up: %v", err)
		}
		log.Println("database migration up")
	default:
		version, err := strconv.Atoi(migrType)
		if err != nil {
			log.Fatalf("failed to parse force version %s: %v", migrType, err)
		}
		if err := m.Force(version); err != nil {
			log.Fatalf("failed tp migrate force %d: %v", version, err)
		}
		log.Printf("database migration to version %d forced", version)
	}

	version, dirty, err := m.Version()
	if err != nil {
		log.Fatalf("failed to get migration version: %v", err)
	}
	log.Printf("database migrations applied successfully, version=%d, dirty=%v", version, dirty)
}
