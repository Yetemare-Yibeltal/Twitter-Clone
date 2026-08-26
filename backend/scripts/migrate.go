// backend/scripts/migrate.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// ======================================================================
// Constants
// ======================================================================

const (
	MigrationsTable = "schema_migrations"
	MigrationsDir   = "backend/migrations"
)

var (
	// Command-line flags
	dbURL     = flag.String("db", "", "Database connection URL")
	migrationDir = flag.String("dir", MigrationsDir, "Migrations directory")
	command   = flag.String("cmd", "up", "Command: up, down, status, create, reset")
	target    = flag.Int("target", 0, "Target migration version")
	createName = flag.String("name", "", "Migration name for create command")
	verbose   = flag.Bool("verbose", false, "Verbose output")
)

// ======================================================================
// Types
// ======================================================================

// Migration represents a single migration file.
type Migration struct {
	Version     int
	Name        string
	UpFile      string
	DownFile    string
	UpSQL       string
	DownSQL     string
	AppliedAt   *time.Time
}

// MigrationStatus represents the status of a migration.
type MigrationStatus struct {
	Version   int
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// ======================================================================
= Main Function
// ======================================================================

func main() {
	flag.Parse()

	if *dbURL == "" {
		// Try to get from environment
		*dbURL = os.Getenv("DATABASE_URL")
		if *dbURL == "" {
			log.Fatal("Database URL is required. Set -db or DATABASE_URL environment variable")
		}
	}

	db, err := sql.Open("postgres", *dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	if *verbose {
		log.Printf("Connected to database successfully")
	}

	// Ensure migrations table exists
	if err := ensureMigrationsTable(db); err != nil {
		log.Fatalf("Failed to ensure migrations table: %v", err)
	}

	switch *command {
	case "up":
		if err := runUp(db); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
	case "down":
		if err := runDown(db); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
	case "status":
		if err := runStatus(db); err != nil {
			log.Fatalf("Status check failed: %v", err)
		}
	case "create":
		if err := runCreate(); err != nil {
			log.Fatalf("Create migration failed: %v", err)
		}
	case "reset":
		if err := runReset(db); err != nil {
			log.Fatalf("Reset failed: %v", err)
		}
	default:
		log.Fatalf("Unknown command: %s. Use up, down, status, create, reset", *command)
	}
}

// ======================================================================
= Database Helpers
// ======================================================================

// ensureMigrationsTable creates the migrations table if it doesn't exist.
func ensureMigrationsTable(db *sql.DB) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, MigrationsTable)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}
	if *verbose {
		log.Printf("Migrations table ensured")
	}
	return nil
}

// getAppliedVersions returns the list of applied migration versions.
func getAppliedVersions(db *sql.DB) (map[int]time.Time, error) {
	query := fmt.Sprintf("SELECT version, applied_at FROM %s ORDER BY version", MigrationsTable)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get applied versions: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]time.Time)
	for rows.Next() {
		var version int
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		applied[version] = appliedAt
	}
	return applied, nil
}

// recordMigration records a migration as applied.
func recordMigration(db *sql.DB, version int, name string) error {
	query := fmt.Sprintf("INSERT INTO %s (version, name, applied_at) VALUES ($1, $2, NOW())", MigrationsTable)
	_, err := db.Exec(query, version, name)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}
	return nil
}

// deleteMigration deletes a migration record.
func deleteMigration(db *sql.DB, version int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE version = $1", MigrationsTable)
	_, err := db.Exec(query, version)
	if err != nil {
		return fmt.Errorf("failed to delete migration: %w", err)
	}
	return nil
}

// ======================================================================
= Migration File Helpers
// ======================================================================

// loadMigrations loads all migration files from the directory.
func loadMigrations() ([]*Migration, error) {
	files, err := filepath.Glob(filepath.Join(*migrationDir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob migration files: %w", err)
	}

	var migrations []*Migration

	for _, upFile := range files {
		base := filepath.Base(upFile)
		// Parse version from filename: 001_init.up.sql
		parts := strings.Split(base, "_")
		if len(parts) < 2 {
			continue
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(strings.Join(parts[1:], "_"), ".up.sql")
		downFile := strings.Replace(upFile, ".up.sql", ".down.sql", 1)

		migration := &Migration{
			Version: version,
			Name:    name,
			UpFile:  upFile,
			DownFile: downFile,
		}

		// Read up SQL
		upSQL, err := os.ReadFile(upFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", upFile, err)
		}
		migration.UpSQL = string(upSQL)

		// Read down SQL (if exists)
		if _, err := os.Stat(downFile); err == nil {
			downSQL, err := os.ReadFile(downFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", downFile, err)
			}
			migration.DownSQL = string(downSQL)
		}

		migrations = append(migrations, migration)
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// ======================================================================
= Command Implementations
// ======================================================================

// runUp applies all pending migrations.
func runUp(db *sql.DB) error {
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	applied, err := getAppliedVersions(db)
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	var targetVersion int
	if *target > 0 {
		targetVersion = *target
	} else {
		// Find the latest migration
		for _, m := range migrations {
			if m.Version > targetVersion {
				targetVersion = m.Version
			}
		}
	}

	if *verbose {
		log.Printf("Target version: %d", targetVersion)
		log.Printf("Applied versions: %v", applied)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, m := range migrations {
		if m.Version > targetVersion {
			break
		}
		if _, ok := applied[m.Version]; ok {
			if *verbose {
				log.Printf("Migration %d already applied, skipping", m.Version)
			}
			continue
		}
		if *verbose {
			log.Printf("Applying migration %d: %s", m.Version, m.Name)
		}
		if err := runMigration(tx, m); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", m.Version, err)
		}
		if err := recordMigration(tx, m.Version, m.Name); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", m.Version, err)
		}
		if *verbose {
			log.Printf("Migration %d applied successfully", m.Version)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("All migrations up to version %d applied successfully", targetVersion)
	return nil
}

// runDown rolls back migrations.
func runDown(db *sql.DB) error {
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	applied, err := getAppliedVersions(db)
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	// Find the latest applied version
	var latestVersion int
	for v := range applied {
		if v > latestVersion {
			latestVersion = v
		}
	}

	if latestVersion == 0 {
		log.Printf("No migrations to rollback")
		return nil
	}

	// Find the migration to rollback
	var target *Migration
	for _, m := range migrations {
		if m.Version == latestVersion {
			target = m
			break
		}
	}

	if target == nil {
		return fmt.Errorf("migration %d not found", latestVersion)
	}

	if *target > 0 {
		// Rollback to target version
		targetVersion := *target
		// Find all migrations above target
		for _, m := range migrations {
			if m.Version > targetVersion && m.Version <= latestVersion {
				if err := rollbackMigration(db, m); err != nil {
					return err
				}
			}
		}
		log.Printf("Rolled back to version %d", targetVersion)
		return nil
	}

	// Rollback one migration
	if err := rollbackMigration(db, target); err != nil {
		return err
	}
	log.Printf("Rolled back migration %d: %s", target.Version, target.Name)
	return nil
}

// rollbackMigration rolls back a single migration.
func rollbackMigration(db *sql.DB, m *Migration) error {
	if m.DownSQL == "" {
		return fmt.Errorf("migration %d has no down SQL", m.Version)
	}
	if *verbose {
		log.Printf("Rolling back migration %d: %s", m.Version, m.Name)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(m.DownSQL)
	if err != nil {
		return fmt.Errorf("failed to execute down SQL: %w", err)
	}
	if err := deleteMigration(tx, m.Version); err != nil {
		return fmt.Errorf("failed to delete migration record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if *verbose {
		log.Printf("Migration %d rolled back successfully", m.Version)
	}
	return nil
}

// runMigration applies a single migration.
func runMigration(tx *sql.Tx, m *Migration) error {
	if m.UpSQL == "" {
		return fmt.Errorf("migration %d has no up SQL", m.Version)
	}
	_, err := tx.Exec(m.UpSQL)
	if err != nil {
		return fmt.Errorf("failed to execute up SQL: %w", err)
	}
	return nil
}

// runStatus shows the status of all migrations.
func runStatus(db *sql.DB) error {
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	applied, err := getAppliedVersions(db)
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	fmt.Println("+---------+----------------------------------------+---------+---------------------+")
	fmt.Println("| VERSION | NAME                                   | APPLIED | APPLIED AT          |")
	fmt.Println("+---------+----------------------------------------+---------+---------------------+")

	for _, m := range migrations {
		appliedAt, ok := applied[m.Version]
		appliedStr := "NO"
		appliedAtStr := "-"
		if ok {
			appliedStr = "YES"
			appliedAtStr = appliedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("| %-7d | %-38s | %-7s | %-19s |\n",
			m.Version, m.Name, appliedStr, appliedAtStr)
	}
	fmt.Println("+---------+----------------------------------------+---------+---------------------+")

	log.Printf("Total migrations: %d, Applied: %d, Pending: %d",
		len(migrations), len(applied), len(migrations)-len(applied))

	return nil
}

// runCreate creates a new migration file.
func runCreate() error {
	if *createName == "" {
		return fmt.Errorf("migration name is required. Use -name flag")
	}

	// Find the next version
	files, err := filepath.Glob(filepath.Join(*migrationDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}

	maxVersion := 0
	for _, f := range files {
		base := filepath.Base(f)
		parts := strings.Split(base, "_")
		if len(parts) < 1 {
			continue
		}
		v, err := strconv.Atoi(parts[0])
		if err == nil && v > maxVersion {
			maxVersion = v
		}
	}
	nextVersion := maxVersion + 1

	// Create the filenames
	upFile := filepath.Join(*migrationDir, fmt.Sprintf("%03d_%s.up.sql", nextVersion, *createName))
	downFile := filepath.Join(*migrationDir, fmt.Sprintf("%03d_%s.down.sql", nextVersion, *createName))

	// Ensure directory exists
	if err := os.MkdirAll(*migrationDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Write up file
	upContent := fmt.Sprintf("-- Migration %d: %s (UP)\n", nextVersion, *createName)
	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		return fmt.Errorf("failed to write up file: %w", err)
	}

	// Write down file
	downContent := fmt.Sprintf("-- Migration %d: %s (DOWN)\n", nextVersion, *createName)
	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		return fmt.Errorf("failed to write down file: %w", err)
	}

	log.Printf("Created migration %d: %s", nextVersion, *createName)
	log.Printf("  Up file: %s", upFile)
	log.Printf("  Down file: %s", downFile)
	return nil
}

// runReset resets the entire database (rollback all migrations and re-apply).
func runReset(db *sql.DB) error {
	log.Printf("Resetting database...")

	// Rollback all migrations
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	applied, err := getAppliedVersions(db)
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	// Rollback from highest to lowest
	for i := len(migrations) - 1; i >= 0; i-- {
		m := migrations[i]
		if _, ok := applied[m.Version]; ok {
			if err := rollbackMigration(db, m); err != nil {
				return fmt.Errorf("failed to rollback migration %d: %w", m.Version, err)
			}
		}
	}

	log.Printf("All migrations rolled back")

	// Now apply all migrations
	if err := runUp(db); err != nil {
		return fmt.Errorf("failed to apply migrations after reset: %w", err)
	}

	log.Printf("Database reset completed successfully")
	return nil
}