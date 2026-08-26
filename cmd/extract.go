package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extrae archivos comprimidos (ZIP, TAR.GZ, RAR, 7Z) masivamente",
	Run: func(cmd *cobra.Command, args []string) {
		if err := RunExtractionLogic(srcDir, destDir); err != nil {
			fmt.Printf("Error fatal en extracción: %v\n", err)
			os.Exit(1)
		}
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

	jobs := make(chan string, 100)
	results := make(chan ExtractResult, 100)
	var wg sync.WaitGroup

	numWorkers := runtime.NumCPU()
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go extractWorker(jobs, results, &wg, dest)
	}

	go func() {
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
		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".zip" || ext == ".gz" || ext == ".tgz" || ext == ".rar" || ext == ".7z" {
				jobs <- path
				count++
			}
		}
		return nil
	})

	if err != nil {
		close(jobs)
		wg.Wait()
		close(results)
		return fmt.Errorf("error leyendo origen: %w", err)
	}

	close(jobs)
	wg.Wait()
	close(results)
	fmt.Printf("Extracción completada. %d archivos procesados.\n", count)
	return nil
}

func extractWorker(jobs <-chan string, results chan<- ExtractResult, wg *sync.WaitGroup, dest string) {
	defer wg.Done()

	for path := range jobs {
		var err error

		baseName := filepath.Base(path)
		folderName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

		if strings.ToLower(filepath.Ext(folderName)) == ".tar" {
			folderName = strings.TrimSuffix(folderName, filepath.Ext(folderName))
		}

		archiveDest := filepath.Join(dest, folderName)
		ext := strings.ToLower(filepath.Ext(path))

		if ext == ".zip" {
			err = extractZip(path, archiveDest)
		} else if ext == ".tar.gz" || ext == ".tgz" || ext == ".gz" {
			err = extractTarGz(path, archiveDest)
		} else if ext == ".rar" {
			err = extractRar(path, archiveDest)
		} else if ext == ".7z" {
			err = extract7z(path, archiveDest)
		} else {
			err = fmt.Errorf("formato no soportado")
		}

		// NUEVO: Si la extracción fue exitosa, aplicamos la Extracción Inteligente
		if err == nil {
			if flattenErr := flattenSingleDirectory(archiveDest); flattenErr != nil {
				// Si falla el aplanamiento lo registramos, pero no detenemos todo
				fmt.Printf("[WARN] No se pudo aplanar %s: %v\n", folderName, flattenErr)
			}
		}

		results <- ExtractResult{Source: path, Err: err}
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
	_, err = io.Copy(dstFile, srcFile)
	return err
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
			if _, err := io.Copy(dstFile, tr); err != nil {
				dstFile.Close()
				return err
			}
			dstFile.Close()
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
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		dstFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(dstFile, rr); err != nil {
			dstFile.Close()
			return err
		}
		dstFile.Close()
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
		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return err
		}
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
