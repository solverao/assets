package database

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate aplica las migraciones pendientes de migrationsFS en orden por
// nombre, registrando las versiones aplicadas en schema_migrations.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY NOT NULL,
		applied_at integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL
	)`); err != nil {
		return fmt.Errorf("creando schema_migrations: %w", err)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("leyendo migraciones: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if applied[name] {
			continue
		}
		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("leyendo %s: %w", name, err)
		}
		if err := applyMigration(db, name, string(content)); err != nil {
			return err
		}
	}
	return nil
}

func appliedVersions(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("consultando schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyMigration(db *sql.DB, name, content string) error {
	statements, err := splitSQL(content)
	if err != nil {
		return fmt.Errorf("parseando %s: %w", name, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("aplicando %s: %w", name, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
		return err
	}
	return tx.Commit()
}

// splitSQL divide un script en statements separados por ';', ignorando
// líneas de comentario ('--').
func splitSQL(content string) ([]string, error) {
	var statements []string
	var current strings.Builder

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}

	if strings.TrimSpace(current.String()) != "" {
		return nil, fmt.Errorf("sentencia sin terminar en ';'")
	}
	return statements, nil
}
