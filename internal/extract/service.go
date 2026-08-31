package extract

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	folder string
	kind   string
}

func NewExtractorService() *ExtractorService {
	return &ExtractorService{}
}

// ExtractAll es el equivalente a tu RunExtractionLogic.
// workers <= 0 activa la detección automática según el tipo de disco (HDD/SSD).
// doSync controla si se hace fsync de archivos y directorios.
// minFree es el espacio libre mínimo requerido en dest (bytes).
// removeSource borra cada comprimido (y sus partes) del origen tras extraerlo con éxito.
// errorDir, si no está vacío, es la cuarentena: ahí se mueven los que fallan y se
// escribe un manifiesto errores.txt con el motivo.
func (s *ExtractorService) ExtractAll(src string, dest string, workers int, doSync bool, minFree int64, removeSource bool, errorDir string) error {
	fmt.Printf("Buscando comprimidos en: %s\n", src)
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return fmt.Errorf("error creando directorio destino: %w", err)
	}

	if err := checkFreeSpace(dest, minFree); err != nil {
		return err
	}

	// Recoge primero la lista de trabajos para poder borrar el origen con
	// seguridad tras procesar (evita borrar durante el WalkDir).
	var jobs []extractJob
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		kind, folder, isCont, ok := classifyArchive(path)
		if !ok || isCont {
			return nil
		}
		relDir, relErr := filepath.Rel(src, filepath.Dir(path))
		if relErr != nil {
			relDir = filepath.Dir(path)
		}
		jobs = append(jobs, extractJob{path: path, relDir: relDir, folder: folder, kind: kind})
		return nil
	})
	if err != nil {
		return fmt.Errorf("error leyendo origen: %w", err)
	}

	jobsCh := make(chan extractJob, len(jobs))
	results := make(chan ExtractResult, len(jobs))
	var wg sync.WaitGroup
	var drainer sync.WaitGroup

	nw := resolveExtractWorkers(dest, workers)

	for w := 1; w <= nw; w++ {
		wg.Add(1)
		go extractWorker(jobsCh, results, &wg, dest, doSync, removeSource, errorDir)
	}

	drainer.Add(1)

	go func() {
		defer drainer.Done()
		var manifest *os.File
		for res := range results {
			if res.Err == nil {
				continue
			}
			fmt.Printf("[ERROR] %s: %v\n", filepath.Base(res.Source), res.Err)
			if errorDir == "" {
				continue
			}
			if manifest == nil {
				if err := os.MkdirAll(errorDir, os.ModePerm); err != nil {
					warnf("No se pudo crear el directorio de errores: %v", err)
					continue
				}
				f, err := os.OpenFile(filepath.Join(errorDir, "errores.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					warnf("No se pudo escribir el manifiesto de errores: %v", err)
					continue
				}
				manifest = f
			}
			rel, _ := filepath.Rel(src, res.Source)
			fmt.Fprintf(manifest, "%s\t%v\n", rel, res.Err)
		}
		if manifest != nil {
			manifest.Close()
		}
	}()

	for _, j := range jobs {
		jobsCh <- j
	}
	close(jobsCh)
	wg.Wait()
	close(results)
	drainer.Wait()
	fmt.Printf("Extracción completada. %d archivos procesados.\n", len(jobs))

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
	kind, _, _, ok := classifyArchive(path)
	if !ok {
		return ""
	}
	return kind
}

var (
	rarSplitRe    = regexp.MustCompile(`(?i)^(.*)\.part(\d+)\.rar$`)
	sevenzSplitRe = regexp.MustCompile(`(?i)^(.*)\.7z\.(\d+)$`)
)

// classifyArchive clasifica un archivo como comprimido. Devuelve el tipo, el
// nombre base (para la carpeta de destino), si es una parte de continuación
// de un comprimido multiparte, y si el formato es soportado.
func classifyArchive(path string) (kind, base string, isContinuation bool, ok bool) {
	name := filepath.Base(path)

	// Multiparte RAR: nombre.partN.rar (N>1 son continuaciones).
	if m := rarSplitRe.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[2])
		return "rar", m[1], n > 1, true
	}
	// Multiparte 7z: nombre.7z.NNN (NNN>001 son continuaciones).
	if m := sevenzSplitRe.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[2])
		return "7z", m[1], n > 1, true
	}

	if strings.HasSuffix(strings.ToLower(name), ".tar.gz") {
		return "targz", trimSuffixFold(name, ".tar.gz"), false, true
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".zip":
		return "zip", trimSuffixFold(name, ".zip"), false, true
	case ".tgz":
		return "targz", trimSuffixFold(name, ".tgz"), false, true
	case ".rar":
		return "rar", trimSuffixFold(name, ".rar"), false, true
	case ".7z":
		return "7z", trimSuffixFold(name, ".7z"), false, true
	}
	return "", "", false, false
}

// trimSuffixFold elimina suffix de s (sin distinguir mayúsculas).
func trimSuffixFold(s, suffix string) string {
	if strings.HasSuffix(strings.ToLower(s), strings.ToLower(suffix)) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func extractWorker(jobs <-chan extractJob, results chan<- ExtractResult, wg *sync.WaitGroup, dest string, doSync bool, removeSource bool, errorDir string) {
	defer wg.Done()

	for job := range jobs {
		archiveDest := filepath.Join(dest, job.relDir, job.folder)
		debugf("Extrayendo %s -> %s", job.path, archiveDest)

		var err error
		var volumes []string

		switch job.kind {
		case "zip":
			err = extractZip(job.path, archiveDest, doSync)
			volumes = []string{job.path}
		case "targz":
			err = extractTarGz(job.path, archiveDest, doSync)
			volumes = []string{job.path}
		case "rar":
			volumes, err = extractRar(job.path, archiveDest, doSync)
		case "7z":
			volumes, err = extract7z(job.path, archiveDest, doSync)
		default:
			err = fmt.Errorf("formato no soportado")
		}

		if err != nil {
			// Limpieza de extracción parcial: no dejamos restos corruptos.
			if rmErr := os.RemoveAll(archiveDest); rmErr != nil {
				debugf("No se pudo limpiar %s: %v", archiveDest, rmErr)
			}
			if errorDir != "" {
				if qErr := quarantineArchive(job, errorDir); qErr != nil {
					warnf("No se pudo mover a cuarentena %s: %v", job.path, qErr)
				}
			}
		} else {
			// Si la extracción fue exitosa, aplicamos la Extracción Inteligente
			if flattenErr := flattenSingleDirectory(archiveDest, doSync); flattenErr != nil {
				// Si falla el aplanamiento lo registramos, pero no detenemos todo
				warnf("No se pudo aplanar %s: %v", job.folder, flattenErr)
			}
			if removeSource {
				for _, v := range volumes {
					if rmErr := os.Remove(v); rmErr != nil && !os.IsNotExist(rmErr) {
						warnf("No se pudo borrar %s: %v", v, rmErr)
					}
				}
			}
		}

		results <- ExtractResult{Source: job.path, Err: err}
	}
}

// quarantineArchive mueve el comprimido fallido (y sus partes hermanas, en
// multiparte) al directorio de cuarentena, conservando la subestructura.
func quarantineArchive(job extractJob, errorDir string) error {
	set := splitSiblings(filepath.Dir(job.path), job.folder, job.kind)
	if len(set) == 0 {
		set = []string{job.path}
	}

	destDir := filepath.Join(errorDir, job.relDir)
	for _, p := range set {
		if err := moveFileQuarantine(p, filepath.Join(destDir, filepath.Base(p))); err != nil {
			return err
		}
	}
	return nil
}

// splitSiblings devuelve las partes de un set multiparte presentes en dir que
// comparten el nombre base, o nil si no es un set multiparte.
func splitSiblings(dir, base, kind string) []string {
	var re *regexp.Regexp
	switch kind {
	case "rar":
		re = rarSplitRe
	case "7z":
		re = sevenzSplitRe
	default:
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var parts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if m := re.FindStringSubmatch(e.Name()); m != nil && m[1] == base {
			parts = append(parts, filepath.Join(dir, e.Name()))
		}
	}
	return parts
}

// moveFileQuarantine mueve src a dst creando directorios padre y con
// fallback de copia+borrado si src y dst están en distintos dispositivos.
func moveFileQuarantine(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		if errors.Is(err, syscall.EXDEV) {
			return copyAndRemoveFile(src, dst)
		}
		return err
	}
	return nil
}

func copyAndRemoveFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
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

func extractRar(src, dest string, doSync bool) ([]string, error) {
	rr, err := rardecode.OpenReader(src, "")
	if err != nil {
		return nil, err
	}
	defer rr.Close()
	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("zipslip: %s", header.Name)
		}
		if header.IsDir {
			if err := mkdirAllNoSymlink(dest, path, 0755); err != nil {
				return nil, err
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
			return nil, err
		}
		dstFile, err := openFileNoFollow(path, sanitizeMode(header.Mode()))
		if err != nil {
			return nil, err
		}
		if err := copyLimited(dstFile, rr); err != nil {
			dstFile.Close()
			return nil, err
		}
		if err := closeFile(dstFile, doSync); err != nil {
			return nil, err
		}
		if doSync {
			syncDir(filepath.Dir(path))
		}
		setFileTimes(path, header.ModificationTime)
	}
	return rr.Volumes(), nil
}

func extract7z(src, dest string, doSync bool) ([]string, error) {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("zipslip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := mkdirAllNoSymlink(dest, path, 0755); err != nil {
				return nil, err
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
			return nil, err
		}
		dstFile, err := openFileNoFollow(path, sanitizeMode(f.Mode()))
		if err != nil {
			return nil, err
		}
		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return nil, err
		}
		err = copyLimited(dstFile, srcFile)
		srcFile.Close()
		if err != nil {
			dstFile.Close()
			return nil, err
		}
		if err := closeFile(dstFile, doSync); err != nil {
			return nil, err
		}
		if doSync {
			syncDir(filepath.Dir(path))
		}
		setFileTimes(path, f.Modified)
	}
	return r.Volumes(), nil
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
