package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func InitDB(dbPath string) (*sql.DB, error) {
	if dbPath != ":memory:" && dbPath != "" {
		if dir := filepath.Dir(dbPath); dir != "." {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return nil, fmt.Errorf("creando directorio de la base de datos: %w", err)
			}
		}
	}

	db, err := sql.Open("sqlite", dbDSN(dbPath))
	if err != nil {
		return nil, err
	}

	// Limitamos a 1 para forzar escrituras secuenciales seguras desde el escritor
	db.SetMaxOpenConns(1)

	if err := Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// dbDSN construye el DSN de modernc.org/sqlite. WAL mode y timeout son
// vitales para evitar "database is locked".
func dbDSN(dbPath string) string {
	if dbPath == ":memory:" {
		return "file::memory:?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	return "file:" + dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
}
