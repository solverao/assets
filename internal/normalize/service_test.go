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

func TestGenerateSlugInternational(t *testing.T) {
	cases := map[string]string{
		"café.txt": "cafe.txt",
		"niño":     "nino",
		"Müller":   "muller",
		"Москва":   "moskva",
		"Ελλάδα":   "ellada",
		"影師":       "ying-shi",
		"foo_bar":  "foo-bar",
	}
	for in, want := range cases {
		if got := Slugify(in, false); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
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
