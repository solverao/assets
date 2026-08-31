package asset

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"asset/internal/database"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestScanDirectoryBuildsTree(t *testing.T) {
	db := setupDB(t)
	svc := NewScannerService(NewSQLiteRepo(db))

	root := t.TempDir()
	sub := filepath.Join(root, "fotos")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.ScanDirectory(root, "local", 2); err != nil {
		t.Fatal(err)
	}

	var folderID int64
	if err := db.QueryRow(`SELECT id FROM assets WHERE type='folder' AND name='fotos'`).Scan(&folderID); err != nil {
		t.Fatalf("carpeta no creada: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM assets WHERE type='file' AND parent_id=?`, folderID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("se esperaban 2 archivos, got %d", count)
	}
}

func TestScanDirectoryDedupBlobs(t *testing.T) {
	db := setupDB(t)
	svc := NewScannerService(NewSQLiteRepo(db))

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.ScanDirectory(root, "local", 2); err != nil {
		t.Fatal(err)
	}

	var blobs int
	if err := db.QueryRow(`SELECT COUNT(*) FROM asset_blobs`).Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if blobs != 1 {
		t.Fatalf("se esperaba 1 blob deduplicado, got %d", blobs)
	}
}

func TestScanDirectorySkipsExisting(t *testing.T) {
	db := setupDB(t)
	svc := NewScannerService(NewSQLiteRepo(db))

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.ScanDirectory(root, "local", 2); err != nil {
		t.Fatal(err)
	}
	if err := svc.ScanDirectory(root, "local", 2); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM assets WHERE type='file'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("se esperaba 1 asset (sin duplicar), got %d", count)
	}
}
