//go:build windows

package extract

import "os"

// openFileNoFollow en Windows no dispone de O_NOFOLLOW; se usa un OpenFile
// estándar (la protección ante symlinks queda cubierta por mkdirAllNoSymlink).
func openFileNoFollow(path string, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
}

// checkFreeSpace en Windows no se implementa; se omite la comprobación.
func checkFreeSpace(dest string, minFree int64) error {
	return nil
}
