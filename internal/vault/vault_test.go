package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryRoundtrip(t *testing.T) {
	dir := t.TempDir()

	r := Registry{Current: "fotos", Vaults: map[string]string{"fotos": "/tmp/fotos/assets.db"}}
	if err := Save(dir, r); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Current != "fotos" {
		t.Fatalf("current = %q, want %q", got.Current, "fotos")
	}
	if got.Vaults["fotos"] != "/tmp/fotos/assets.db" {
		t.Fatalf("vault path = %q", got.Vaults["fotos"])
	}
}

func TestLoadMissing(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Vaults) != 0 || r.Current != "" {
		t.Fatalf("se esperaba un registro vacío, got %+v", r)
	}
}

func TestDefaultDBPath(t *testing.T) {
	got := DefaultDBPath("/cfg", "fotos")
	want := filepath.Join("/cfg", "vaults", "fotos", "assets.db")
	if got != want {
		t.Fatalf("DefaultDBPath = %q, want %q", got, want)
	}
}

func TestRegistryPath(t *testing.T) {
	if got := RegistryPath("/cfg"); got != filepath.Join("/cfg", "vaults.json") {
		t.Fatalf("RegistryPath = %q", got)
	}
	if _, err := os.Stat(RegistryPath(t.TempDir())); !os.IsNotExist(err) {
		t.Fatal("el registro no debería existir aún")
	}
}
