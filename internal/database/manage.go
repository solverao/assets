package database

import (
	"database/sql"
	"fmt"
	"os"
)

// DBInfo describe el estado de una base de datos SQLite.
type DBInfo struct {
	Path       string
	Exists     bool
	Size       int64
	Migrations int
	Assets     int64
	Blobs      int64
}

// Create crea la base de datos en path (creando el directorio padre si hace
// falta) y aplica las migraciones.
func Create(path string) error {
	db, err := InitDB(path)
	if err != nil {
		return err
	}
	return db.Close()
}

// Info abre path sin migrar y recoge estadísticas básicas. No crea el archivo
// si no existe.
func Info(path string) (DBInfo, error) {
	info := DBInfo{Path: path}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return info, nil
	}
	if fi, err := os.Stat(path); err == nil {
		info.Size = fi.Size()
	}
	info.Exists = true

	db, err := openRaw(path)
	if err != nil {
		return info, err
	}
	defer db.Close()

	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&info.Migrations); err != nil {
		// Esquema aún sin migrar; se trata como 0.
		info.Migrations = 0
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM assets`).Scan(&info.Assets)
	_ = db.QueryRow(`SELECT COUNT(*) FROM asset_blobs`).Scan(&info.Blobs)

	return info, nil
}

// ListMigrations devuelve las versiones de migración ya aplicadas.
func ListMigrations(path string) ([]string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	db, err := openRaw(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// Delete borra la base de datos y sus archivos auxiliares WAL/SHM.
func Delete(path string) error {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("borrando %s: %w", p, err)
		}
	}
	return nil
}

// openRaw abre la base de datos sin ejecutar migraciones.
func openRaw(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}
