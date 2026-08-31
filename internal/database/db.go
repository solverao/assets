package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(dbPath string) (*sql.DB, error) {
	if dbPath != ":memory:" && dbPath != "" {
		if dir := filepath.Dir(dbPath); dir != "." {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return nil, fmt.Errorf("creando directorio de la base de datos: %w", err)
			}
		}
	}

	// WAL mode y timeout son vitales para evitar "database is locked"
	dsn := fmt.Sprintf("file:%s?_fk=1&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite3", dsn)
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
