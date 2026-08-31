package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateInfoDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "assets.db")

	if err := Create(path); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("la BD no se creó: %v", err)
	}

	info, err := Info(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Exists {
		t.Fatal("info.Exists debería ser true")
	}
	if info.Migrations == 0 {
		t.Fatal("se esperaba al menos 1 migración aplicada")
	}

	if err := Delete(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("la BD debería haberse borrado: %v", err)
	}
}

func TestInfoNonexistent(t *testing.T) {
	info, err := Info(filepath.Join(t.TempDir(), "no-existe.db"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Exists {
		t.Fatal("no debería existir")
	}
}

func TestListMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "assets.db")
	if err := Create(path); err != nil {
		t.Fatal(err)
	}

	versions, err := ListMigrations(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("se esperaban migraciones listadas")
	}
	if versions[0] != "001_init_schema.sql" {
		t.Fatalf("primera migración = %q, want %q", versions[0], "001_init_schema.sql")
	}
}
