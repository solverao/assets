package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCalculateChecksum(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := calculateChecksum(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("checksum = %s, want %s", got, want)
	}
}

func TestRunChecksumLogicWritesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RunChecksumLogic(dir, "checksums.txt"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		t.Fatalf("checksums.txt no encontrado: %v", err)
	}
	if !strings.Contains(string(data), "a.txt") {
		t.Fatalf("checksums.txt no contiene la ruta relativa: %q", data)
	}
}
