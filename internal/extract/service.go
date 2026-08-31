package extract

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode"
	"golang.org/x/sys/unix"
)

// ExtractorService encapsula la lógica de descompresión
type ExtractorService struct {
	// Aquí podrías inyectar un Logger si lo necesitaras en el futuro
}

type ExtractResult struct {
	Source string
	Err    error
}

type extractJob struct {
	path   string
	relDir string
}

func NewExtractorService() *ExtractorService {
	return &ExtractorService{}
}

// ExtractAll es el equivalente a tu RunExtractionLogic.
// workers <= 0 activa la detección automática según el tipo de disco (HDD/SSD).
// sync controla si se hace fsync de archivos y directorios.
// minFree es el espacio libre mínimo requerido en dest (bytes).
func (s *ExtractorService) ExtractAll(src string, dest string, workers int, doSync bool, minFree int64) error {
	fmt.Printf("Buscando comprimidos en: %s\n", src)
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return fmt.Errorf("error creando directorio destino: %w", err)
	}

	if err := checkFreeSpace(dest, minFree); err != nil {
		return err
	}

	jobs := make(chan extractJob, 100)
	results := make(chan ExtractResult, 100)
	var wg sync.WaitGroup
	var drainer sync.WaitGroup

	nw := resolveExtractWorkers(dest, workers)

	for w := 1; w <= nw; w++ {
		wg.Add(1)
		go extractWorker(jobs, results, &wg, dest, doSync)
	}

	drainer.Add(1)

	go func() {
		defer drainer.Done()
		for res := range results {
			if res.Err != nil {
				fmt.Printf("[ERROR] %s: %v\n", filepath.Base(res.Source), res.Err)
			}
		}
	}()

	count := 0
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && detectArchiveType(path) != "" {
			relDir, relErr := filepath.Rel(src, filepath.Dir(path))
			if relErr != nil {
				relDir = filepath.Dir(path)
			}
			jobs <- extractJob{path: path, relDir: relDir}
			count++
		}
		return nil
	})

	if err != nil {
		close(jobs)
		wg.Wait()
		close(results)
		drainer.Wait()
		return fmt.Errorf("error leyendo origen: %w", err)
	}

	close(jobs)
	wg.Wait()
	close(results)
	drainer.Wait()
	fmt.Printf("Extracción completada. %d archivos procesados.\n", count)

	return nil
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

// resolveExtractWorkers decide cuántos workers usar. Si workers > 0 se respeta;
// si es 0 se detecta el tipo de disco (1 en HDD, NumCPU en SSD).
func resolveExtractWorkers(dest string, workers int) int {
	if workers > 0 {
		return workers
	}
	if rot, err := isRotational(dest); err == nil {
		if rot {
			return 1
		}
		return runtime.NumCPU()
	}
	// Sin capacidad de detección, se usa un valor conservador.
	if n := runtime.NumCPU(); n > 2 {
		return 2
	}
	return runtime.NumCPU()
}

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

// detectArchiveType devuelve "zip", "targz", "rar" o "7z" según la
// extensión del archivo, o "" si no es un formato soportado.
func detectArchiveType(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") {
		return "targz"
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".zip":
		return "zip"
	case ".tgz":
		return "targz"
	case ".rar":
		return "rar"
	case ".7z":
		return "7z"
	}
	return ""
}

func extractWorker(jobs <-chan extractJob, results chan<- ExtractResult, wg *sync.WaitGroup, dest string, doSync bool) {
	defer wg.Done()

	for job := range jobs {
		var err error

		baseName := filepath.Base(job.path)
		folderName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

		if strings.ToLower(filepath.Ext(folderName)) == ".tar" {
			folderName = strings.TrimSuffix(folderName, filepath.Ext(folderName))
		}

		archiveDest := filepath.Join(dest, job.relDir, folderName)
		debugf("Extrayendo %s -> %s", job.path, archiveDest)

		switch detectArchiveType(job.path) {
		case "zip":
			err = extractZip(job.path, archiveDest, doSync)
		case "targz":
			err = extractTarGz(job.path, archiveDest, doSync)
		case "rar":
			err = extractRar(job.path, archiveDest, doSync)
		case "7z":
			err = extract7z(job.path, archiveDest, doSync)
		default:
			err = fmt.Errorf("formato no soportado")
		}

		if err != nil {
			// Limpieza de extracción parcial: no dejamos restos corruptos.
			if rmErr := os.RemoveAll(archiveDest); rmErr != nil {
				debugf("No se pudo limpiar %s: %v", archiveDest, rmErr)
			}
		} else {
			// Si la extracción fue exitosa, aplicamos la Extracción Inteligente
			if flattenErr := flattenSingleDirectory(archiveDest, doSync); flattenErr != nil {
				// Si falla el aplanamiento lo registramos, pero no detenemos todo
				warnf("No se pudo aplanar %s: %v", folderName, flattenErr)
			}
		}

		results <- ExtractResult{Source: job.path, Err: err}
	}
}

// maxExtractedFileSize limita el tamaño descomprimido por archivo para
// mitigar ataques de tipo "zip bomb".
const maxExtractedFileSize = int64(10 << 30) // 10 GiB

// limitedWriter corta la escritura cuando se supera el tamaño máximo.
type limitedWriter struct {
	w       io.Writer
	max     int64
	written int64
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.written+int64(len(p)) > l.max {
		return 0, fmt.Errorf("límite de tamaño excedido (%d bytes)", l.max)
	}
	n, err := l.w.Write(p)
	l.written += int64(n)
	return n, err
}

// copyLimited copia src a dst deteniéndose con error si se supera max bytes.
func copyLimited(dst io.Writer, src io.Reader) error {
	lw := &limitedWriter{w: dst, max: maxExtractedFileSize}
	_, err := io.Copy(lw, src)
	return err
}

// setFileTimes fija el timestamp de modificación si está disponible.
func setFileTimes(path string, mtime time.Time) {
	if mtime.IsZero() {
		return
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		debugf("No se pudo fijar timestamps para %s: %v", path, err)
	}
}

// sanitizeMode elimina bits peligrosos (setuid/setgid/sticky) del modo.
func sanitizeMode(mode os.FileMode) os.FileMode {
	return mode.Perm()
}

// openFileNoFollow crea el archivo de salida sin seguir symlinks (O_NOFOLLOW)
// para evitar que un symlink preexistente en dest redirija la escritura.
func openFileNoFollow(path string, mode os.FileMode) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

// closeFile sincroniza (fsync) y cierra el archivo si se solicita durabilidad.
func closeFile(f *os.File, doSync bool) error {
	if doSync {
		if err := f.Sync(); err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}

// syncDir fuerza la persistencia de las entradas de un directorio.
func syncDir(path string) {
	if path == "" {
		return
	}
	d, err := os.Open(path)
	if err != nil {
		debugf("No se pudo abrir directorio para sync %s: %v", path, err)
		return
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		debugf("No se pudo sincronizar directorio %s: %v", path, err)
	}
}

// mkdirAllNoSymlink crea los directorios intermedios de path (que debe estar
// bajo root) rechazando que cualquier componente existente sea un symlink
// (mitiga TOCTOU). Los componentes anteriores a root no se comprueban.
func mkdirAllNoSymlink(root, path string, perm os.FileMode) error {
	// root es el directorio de extracción (de confianza); lo creamos si falta.
	if err := os.MkdirAll(root, perm); err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("ruta fuera del destino: %s", path)
	}
	cur := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("componente de ruta es un symlink: %s", cur)
			}
			if !info.IsDir() {
				return fmt.Errorf("componente de ruta no es un directorio: %s", cur)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(cur, perm); err != nil {
			return err
		}
	}
	return nil
}

func extractZip(src, dest string, doSync bool) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if err := extractZipFile(f, dest, doSync); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, dest string, doSync bool) error {
	path := filepath.Join(dest, f.Name)
	if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
		return fmt.Errorf("zipslip: %s", f.Name)
	}
	if f.Mode()&os.ModeSymlink != 0 {
		debugf("Saltando symlink: %s", f.Name)
		return nil
	}
	if f.FileInfo().IsDir() {
		if err := mkdirAllNoSymlink(dest, path, os.ModePerm); err != nil {
			return err
		}
		if doSync {
			syncDir(filepath.Dir(path))
		}
		return nil
	}
	if err := mkdirAllNoSymlink(dest, filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}
	dstFile, err := openFileNoFollow(path, sanitizeMode(f.Mode()))
	if err != nil {
		return err
	}
	srcFile, err := f.Open()
	if err != nil {
		dstFile.Close()
		return err
	}
	if err := copyLimited(dstFile, srcFile); err != nil {
		srcFile.Close()
		dstFile.Close()
		return err
	}
	srcFile.Close()
	if err := closeFile(dstFile, doSync); err != nil {
		return err
	}
	if doSync {
		syncDir(filepath.Dir(path))
	}
	setFileTimes(path, f.Modified)
	return nil
}

func extractTarGz(src, dest string, doSync bool) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("zipslip: %s", header.Name)
		}
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			debugf("Saltando symlink: %s", header.Name)
			continue
		}
		if header.Typeflag == tar.TypeDir {
			if err := mkdirAllNoSymlink(dest, path, 0755); err != nil {
				return err
			}
			if doSync {
				syncDir(filepath.Dir(path))
			}
		} else if header.Typeflag == tar.TypeReg {
			if err := mkdirAllNoSymlink(dest, filepath.Dir(path), 0755); err != nil {
				return err
			}
			dstFile, err := openFileNoFollow(path, sanitizeMode(os.FileMode(header.Mode)))
			if err != nil {
				return err
			}
			if err := copyLimited(dstFile, tr); err != nil {
				dstFile.Close()
				return err
			}
			if err := closeFile(dstFile, doSync); err != nil {
				return err
			}
			if doSync {
				syncDir(filepath.Dir(path))
			}
			setFileTimes(path, header.ModTime)
		}
	}
	return nil
}

func extractRar(src, dest string, doSync bool) error {
	rr, err := rardecode.OpenReader(src, "")
	if err != nil {
		return err
	}
	defer rr.Close()
	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("zipslip: %s", header.Name)
		}
		if header.IsDir {
			if err := mkdirAllNoSymlink(dest, path, 0755); err != nil {
				return err
			}
			if doSync {
				syncDir(filepath.Dir(path))
			}
			continue
		}
		if header.Mode()&os.ModeSymlink != 0 {
			debugf("Saltando symlink: %s", header.Name)
			continue
		}
		if err := mkdirAllNoSymlink(dest, filepath.Dir(path), 0755); err != nil {
			return err
		}
		dstFile, err := openFileNoFollow(path, sanitizeMode(header.Mode()))
		if err != nil {
			return err
		}
		if err := copyLimited(dstFile, rr); err != nil {
			dstFile.Close()
			return err
		}
		if err := closeFile(dstFile, doSync); err != nil {
			return err
		}
		if doSync {
			syncDir(filepath.Dir(path))
		}
		setFileTimes(path, header.ModificationTime)
	}
	return nil
}

func extract7z(src, dest string, doSync bool) error {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("zipslip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := mkdirAllNoSymlink(dest, path, 0755); err != nil {
				return err
			}
			if doSync {
				syncDir(filepath.Dir(path))
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			debugf("Saltando symlink: %s", f.Name)
			continue
		}
		if err := mkdirAllNoSymlink(dest, filepath.Dir(path), 0755); err != nil {
			return err
		}
		dstFile, err := openFileNoFollow(path, sanitizeMode(f.Mode()))
		if err != nil {
			return err
		}
		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}
		err = copyLimited(dstFile, srcFile)
		srcFile.Close()
		if err != nil {
			dstFile.Close()
			return err
		}
		if err := closeFile(dstFile, doSync); err != nil {
			return err
		}
		if doSync {
			syncDir(filepath.Dir(path))
		}
		setFileTimes(path, f.Modified)
	}
	return nil
}

// flattenSingleDirectory revisa si una carpeta contiene una sola subcarpeta.
// Si es así, "sube" el contenido un nivel para evitar carpetas dobles.
func flattenSingleDirectory(targetDir string, doSync bool) error {
	// Leemos qué hay dentro de la carpeta que acabamos de extraer
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return err
	}

	// Si hay exactamente UN solo elemento, y es un directorio...
	if len(entries) == 1 && entries[0].IsDir() {
		// Generamos un nombre temporal seguro
		tempDir := targetDir + "_tmp"
		for {
			if _, err := os.Stat(tempDir); os.IsNotExist(err) {
				break
			}
			tempDir += "1"
		}

		// 1. Renombramos la carpeta base a algo temporal
		// /destino/proyecto-2026 -> /destino/proyecto-2026_tmp
		if err := os.Rename(targetDir, tempDir); err != nil {
			return err
		}

		// 2. Identificamos la carpeta interna que quedó dentro del temporal
		// /destino/proyecto-2026_tmp/proyecto-2026
		innerDir := filepath.Join(tempDir, entries[0].Name())

		// 3. Renombramos la carpeta interna usando el nombre original base
		// /destino/proyecto-2026_tmp/proyecto-2026 -> /destino/proyecto-2026
		if err := os.Rename(innerDir, targetDir); err != nil {
			// Rollback de seguridad por si algo falla
			os.Rename(tempDir, targetDir)
			return err
		}

		// 4. Borramos la envoltura temporal (que ahora ya está vacía)
		os.Remove(tempDir)

		if doSync {
			syncDir(filepath.Dir(targetDir))
		}
	}

	return nil
}

func debugf(format string, args ...interface{}) {
	// Descomenta la siguiente línea si quieres ver los logs de depuración
	// log.Printf("[DEBUG] "+format, args...)
}

func warnf(format string, args ...interface{}) {
	log.Printf("[WARNING] "+format, args...)
}
