package database

import (
	"testing"
)

func TestMigrateCreatesTables(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	tables := []string{"assets", "asset_blobs", "asset_thumbnails", "schema_migrations"}
	for _, name := range tables {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n); err != nil {
			t.Fatalf("consultando %s: %v", name, err)
		}
		if n != 1 {
			t.Fatalf("tabla %s no creada", name)
		}
	}

	var versions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions == 0 {
		t.Fatal("schema_migrations vacía")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("segunda migración: %v", err)
	}

	var versions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 1 {
		t.Fatalf("se esperaba 1 versión, got %d", versions)
	}
}

func TestSplitSQL(t *testing.T) {
	content := "-- comentario\nCREATE TABLE a (id int);\n\nCREATE TABLE b (id int);\n"
	stmts, err := splitSQL(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 2 {
		t.Fatalf("se esperaban 2 statements, got %d", len(stmts))
	}
}
