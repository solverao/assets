package cmd

import (
	"asset/internal/checksum"
	"asset/internal/extract"
	"asset/internal/normalize"
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

	if err := extract.NewExtractorService().ExtractAll(src, dest, 2, false, 0); err != nil {
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
