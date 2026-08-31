package database

import (
	"strings"
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

func TestSplitSQLTrigger(t *testing.T) {
	content := "CREATE TRIGGER t AFTER INSERT ON x BEGIN\n\tINSERT INTO f(a) VALUES (';');\n\tINSERT INTO f(a) VALUES (2);\nEND;\n"
	stmts, err := splitSQL(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 {
		t.Fatalf("se esperaba 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "END;") {
		t.Fatalf("el trigger debería conservar su END;: %q", stmts[0])
	}
}

func TestSplitSQLBlockCommentAndString(t *testing.T) {
	content := "/* bloque ; con ; */\nCREATE TABLE a (v text DEFAULT ';');\n"
	stmts, err := splitSQL(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 {
		t.Fatalf("se esperaba 1 statement, got %d", len(stmts))
	}
}

func TestMigrateDetectsDrift(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`UPDATE schema_migrations SET checksum = 'bogus'`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err == nil {
		t.Fatal("se esperaba error por drift de migración")
	}
}
