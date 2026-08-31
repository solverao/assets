//go:build unix

package extract

import "testing"

func TestCheckFreeSpace(t *testing.T) {
	dest := t.TempDir()
	if err := checkFreeSpace(dest, 0); err != nil {
		t.Fatalf("minFree=0 debería desactivar la comprobación: %v", err)
	}
	if err := checkFreeSpace(dest, int64(1)<<60); err == nil {
		t.Fatal("se esperaba error por espacio insuficiente")
	}
}
