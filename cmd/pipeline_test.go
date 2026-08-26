package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFileRobustRename(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "b.txt")

	if err := moveFileRobust(src, dest); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("origen debería haber desaparecido")
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("destino no existe: %v", err)
	}
}

func TestCopyAndRemovePreservesMode(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(src, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "b.txt")

	if err := copyAndRemove(src, dest); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permisos = %v, want %v", info.Mode().Perm(), os.FileMode(0600))
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("origen debería haber desaparecido")
	}
}
