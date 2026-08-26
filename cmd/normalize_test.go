package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSlugFile(t *testing.T) {
	if got := generateSlug("Mi Archivo.JPG", false); got != "mi-archivo.jpg" {
		t.Fatalf("got %q, want %q", got, "mi-archivo.jpg")
	}
}

func TestGenerateSlugDir(t *testing.T) {
	if got := generateSlug("Mi Carpeta", true); got != "mi-carpeta" {
		t.Fatalf("got %q, want %q", got, "mi-carpeta")
	}
}

func TestGenerateSlugEmpty(t *testing.T) {
	if got := generateSlug("!!", false); got != "item" {
		t.Fatalf("got %q, want %q", got, "item")
	}
}

func TestRunNormalizationLogic(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "Mi Carpeta")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "Mi Archivo.TXT"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RunNormalizationLogic(dir, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "mi-carpeta", "mi-archivo.txt")); err != nil {
		t.Fatalf("archivo renombrado no encontrado: %v", err)
	}
}
