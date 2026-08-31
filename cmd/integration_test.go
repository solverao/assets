package cmd

import (
	"asset/internal/checksum"
	"asset/internal/database"
	"asset/internal/extract"
	"asset/internal/normalize"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationPipeline(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	sub := filepath.Join(src, "paquetes")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	makeZip(t, filepath.Join(sub, "Proyecto Demo.zip"), map[string]string{
		"Proyecto Demo/archivo Uno.txt": "contenido",
	})

	if err := extract.NewExtractorService().ExtractAll(context.Background(), extract.ExtractOptions{
		Src: src, Dest: dest, Workers: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if err := normalize.NewNormalizerService().NormalizeAll(dest, false); err != nil {
		t.Fatal(err)
	}
	if err := checksum.NewChecksumService().ChecksumAll(dest, "checksums.txt", 2); err != nil {
		t.Fatal(err)
	}

	final := filepath.Join(dest, "paquetes", "proyecto-demo", "archivo-uno.txt")
	data, err := os.ReadFile(final)
	if err != nil {
		t.Fatalf("archivo final no encontrado: %v", err)
	}
	if string(data) != "contenido" {
		t.Fatalf("contenido = %q, want %q", data, "contenido")
	}

	cs, err := os.ReadFile(filepath.Join(dest, "checksums.txt"))
	if err != nil {
		t.Fatalf("checksums.txt no encontrado: %v", err)
	}
	if !strings.Contains(string(cs), "proyecto-demo/archivo-uno.txt") {
		t.Fatalf("checksums.txt no contiene la ruta esperada: %q", cs)
	}
}

func TestIngestProcessesAndIndexes(t *testing.T) {
	src := t.TempDir()
	sub := filepath.Join(src, "paquetes")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	makeZip(t, filepath.Join(sub, "Proyecto Demo.zip"), map[string]string{
		"Proyecto Demo/archivo Uno.txt": "contenido",
	})

	dest := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "assets.db")

	if err := runIngest(context.Background(),
		extract.NewExtractorService(),
		normalize.NewNormalizerService(),
		checksum.NewChecksumService(),
		src, dest, dbPath, 0, false, "", "", false); err != nil {
		t.Fatal(err)
	}

	final := filepath.Join(dest, "paquetes", "proyecto-demo", "archivo-uno.txt")
	if _, err := os.Stat(final); err != nil {
		t.Fatalf("archivo final no encontrado: %v", err)
	}

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM assets WHERE type='file' AND slug='archivo-uno.txt'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("se esperaba 1 asset indexado, got %d", count)
	}
}

func TestIngestIncludesLooseFiles(t *testing.T) {
	src := t.TempDir()
	sub := filepath.Join(src, "paquetes")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	makeZip(t, filepath.Join(sub, "Demo.zip"), map[string]string{"Demo/f.txt": "hola"})
	if err := os.WriteFile(filepath.Join(sub, "nota.txt"), []byte("suelto"), 0644); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "assets.db")

	if err := runIngest(context.Background(),
		extract.NewExtractorService(),
		normalize.NewNormalizerService(),
		checksum.NewChecksumService(),
		src, dest, dbPath, 0, false, "", "", false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dest, "paquetes", "nota.txt")); err != nil {
		t.Fatalf("suelto no copiado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "paquetes", "demo", "f.txt")); err != nil {
		t.Fatalf("extraído no encontrado: %v", err)
	}

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, slug := range []string{"nota.txt", "f.txt"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM assets WHERE type='file' AND slug=?`, slug).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("slug %q no indexado (got %d)", slug, n)
		}
	}
}
