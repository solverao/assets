//go:build linux

package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// isRotational indica si el dispositivo que aloja dir es rotacional (HDD)
// leyendo /sys/dev/block/<major>:<minor>/queue/rotational.
func isRotational(dir string) (bool, error) {
	var st unix.Stat_t
	if err := unix.Stat(dir, &st); err != nil {
		return false, err
	}
	link := fmt.Sprintf("/sys/dev/block/%d:%d", unix.Major(st.Dev), unix.Minor(st.Dev))
	target, err := os.Readlink(link)
	if err != nil {
		return false, err
	}
	abs := target
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(filepath.Dir(link), target)
	}
	// Las particiones viven bajo el dispositivo de bloque; subimos hasta
	// encontrar el directorio que contiene queue/rotational.
	for i := 0; i < 8; i++ {
		rot := filepath.Join(abs, "queue", "rotational")
		if data, err := os.ReadFile(rot); err == nil {
			return strings.TrimSpace(string(data)) == "1", nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return false, fmt.Errorf("no se pudo determinar si %s es rotacional", dir)
}
