//go:build unix

package extract

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openFileNoFollow crea el archivo de salida sin seguir symlinks (O_NOFOLLOW)
// para evitar que un symlink preexistente en dest redirija la escritura.
func openFileNoFollow(path string, mode os.FileMode) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

// checkFreeSpace aborta si el espacio libre en dest queda por debajo de minFree.
func checkFreeSpace(dest string, minFree int64) error {
	if minFree <= 0 {
		return nil
	}
	var st unix.Statfs_t
	if err := unix.Statfs(dest, &st); err != nil {
		return fmt.Errorf("no se pudo comprobar el espacio libre: %w", err)
	}
	free := int64(st.Bavail) * int64(st.Bsize)
	if free < minFree {
		return fmt.Errorf("espacio libre insuficiente en %s: %d bytes disponibles (< %d requeridos)", dest, free, minFree)
	}
	return nil
}
