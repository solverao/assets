package normalize

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSlugFile(t *testing.T) {
	if got := Slugify("Mi Archivo.JPG", false); got != "mi-archivo.jpg" {
		t.Fatalf("got %q, want %q", got, "mi-archivo.jpg")
	}
}

func TestGenerateSlugDir(t *testing.T) {
	if got := Slugify("Mi Carpeta", true); got != "mi-carpeta" {
		t.Fatalf("got %q, want %q", got, "mi-carpeta")
	}
}

func TestGenerateSlugEmpty(t *testing.T) {
	if got := Slugify("!!", false); got != "item" {
		t.Fatalf("got %q, want %q", got, "item")
	}
}

func TestNormalizeAll(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "Mi Carpeta")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "Mi Archivo.TXT"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := NewNormalizerService().NormalizeAll(dir, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "mi-carpeta", "mi-archivo.txt")); err != nil {
		t.Fatalf("archivo renombrado no encontrado: %v", err)
	}
}
