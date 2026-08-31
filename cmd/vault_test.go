package cmd

import (
	"path/filepath"
	"testing"

	"asset/internal/vault"
)

func TestResolveDBPathFromVault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vaults", "fotos", "assets.db")
	r := vault.Registry{Current: "fotos", Vaults: map[string]string{"fotos": path}}
	if err := vault.Save(dir, r); err != nil {
		t.Fatal(err)
	}
	if got := resolveDBPathFrom(dir); got != path {
		t.Fatalf("resolveDBPathFrom = %q, want %q", got, path)
	}
}

func TestResolveDBPathFromEmpty(t *testing.T) {
	if got := resolveDBPathFrom(t.TempDir()); got != "assets.db" {
		t.Fatalf("resolveDBPathFrom = %q, want assets.db", got)
	}
}

func TestResolveDBPathEnv(t *testing.T) {
	t.Setenv("ASSET_DB", "/tmp/env.db")
	if got := resolveDBPath(); got != "/tmp/env.db" {
		t.Fatalf("resolveDBPath = %q, want /tmp/env.db", got)
	}
}
