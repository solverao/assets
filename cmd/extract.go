package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode"
	"github.com/spf13/cobra"
)

var srcDir string
var destDir string

type ExtractResult struct {
	Source string
	Err    error
}

type extractJob struct {
	path   string
	relDir string
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

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extrae archivos comprimidos (ZIP, TAR.GZ, RAR, 7Z) masivamente",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunExtractionLogic(srcDir, destDir)
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().StringVarP(&srcDir, "src", "s", "", "Directorio origen (requerido)")
	extractCmd.Flags().StringVarP(&destDir, "dest", "d", "", "Directorio destino (requerido)")
	extractCmd.MarkFlagRequired("src")
	extractCmd.MarkFlagRequired("dest")
}

func RunExtractionLogic(src string, dest string) error {
	fmt.Printf("Buscando comprimidos en: %s\n", src)
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return fmt.Errorf("error creando directorio destino: %w", err)
	}

	jobs := make(chan extractJob, 100)
	results := make(chan ExtractResult, 100)
	var wg sync.WaitGroup
	var drainer sync.WaitGroup

	nw := numWorkers()
	for w := 1; w <= nw; w++ {
		wg.Add(1)
		go extractWorker(jobs, results, &wg, dest)
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

func extractWorker(jobs <-chan extractJob, results chan<- ExtractResult, wg *sync.WaitGroup, dest string) {
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
			err = extractZip(job.path, archiveDest)
		case "targz":
			err = extractTarGz(job.path, archiveDest)
		case "rar":
			err = extractRar(job.path, archiveDest)
		case "7z":
			err = extract7z(job.path, archiveDest)
		default:
			err = fmt.Errorf("formato no soportado")
		}

		// Si la extracción fue exitosa, aplicamos la Extracción Inteligente
		if err == nil {
			if flattenErr := flattenSingleDirectory(archiveDest); flattenErr != nil {
				// Si falla el aplanamiento lo registramos, pero no detenemos todo
				warnf("No se pudo aplanar %s: %v", folderName, flattenErr)
			}
		}

		results <- ExtractResult{Source: job.path, Err: err}
	}
}

// maxExtractedFileSize limita el tamaño descomprimido por archivo para
// mitigar ataques de tipo "zip bomb".
const maxExtractedFileSize = int64(2 << 30) // 2 GiB

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

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if err := extractZipFile(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, dest string) error {
	path := filepath.Join(dest, f.Name)
	if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
		return fmt.Errorf("zipslip: %s", f.Name)
	}
	if f.Mode()&os.ModeSymlink != 0 {
		debugf("Saltando symlink: %s", f.Name)
		return nil
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(path, os.ModePerm)
	}
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}
	dstFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()
	srcFile, err := f.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()
	if err := copyLimited(dstFile, srcFile); err != nil {
		return err
	}
	setFileTimes(path, f.Modified)
	return nil
}

func extractTarGz(src, dest string) error {
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
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		} else if header.Typeflag == tar.TypeReg {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			dstFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if err := copyLimited(dstFile, tr); err != nil {
				dstFile.Close()
				return err
			}
			dstFile.Close()
			setFileTimes(path, header.ModTime)
		}
	}
	return nil
}

func extractRar(src, dest string) error {
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
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}
		if header.Mode()&os.ModeSymlink != 0 {
			debugf("Saltando symlink: %s", header.Name)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		dstFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.Mode())
		if err != nil {
			return err
		}
		if err := copyLimited(dstFile, rr); err != nil {
			dstFile.Close()
			return err
		}
		dstFile.Close()
		setFileTimes(path, header.ModificationTime)
	}
	return nil
}

func extract7z(src, dest string) error {
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
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			debugf("Saltando symlink: %s", f.Name)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		dstFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
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
		dstFile.Close()
		if err != nil {
			return err
		}
		setFileTimes(path, f.Modified)
	}
	return nil
}

// flattenSingleDirectory revisa si una carpeta contiene una sola subcarpeta.
// Si es así, "sube" el contenido un nivel para evitar carpetas dobles.
func flattenSingleDirectory(targetDir string) error {
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
	}

	return nil
}
