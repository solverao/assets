package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var checkDir string

type FileResult struct {
	Path     string
	Checksum string
	Err      error
}

var checksumCmd = &cobra.Command{
	Use:   "checksum",
	Short: "Calcula los checksums SHA-256 masivamente",
	Run: func(cmd *cobra.Command, args []string) {
		if err := RunChecksumLogic(checkDir); err != nil {
			fmt.Printf("Error fatal en checksums: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(checksumCmd)
	checksumCmd.Flags().StringVarP(&checkDir, "dir", "d", "", "Directorio a procesar (requerido)")
	checksumCmd.MarkFlagRequired("dir")
}

func RunChecksumLogic(targetDir string) error {
	var totalFiles int
	var filesToProcess []string

	err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			filesToProcess = append(filesToProcess, path)
			totalFiles++
		}
		return nil
	})

	if err != nil {
		return err
	}
	if totalFiles == 0 {
		fmt.Println("No hay archivos para procesar checksums.")
		return nil
	}

	bar := progressbar.Default(int64(totalFiles), "Calculando Checksums")
	jobs := make(chan string, totalFiles)
	results := make(chan FileResult, totalFiles)
	var wg sync.WaitGroup

	numWorkers := runtime.NumCPU()
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go checksumWorker(jobs, results, &wg)
	}

	go func() {
		for res := range results {
			bar.Add(1)
			if res.Err != nil {
				bar.Describe(fmt.Sprintf("[ERROR] %s", filepath.Base(res.Path)))
			}
		}
	}()

	for _, path := range filesToProcess {
		jobs <- path
	}
	close(jobs)
	wg.Wait()
	close(results)

	fmt.Println("\nCálculo de checksums completado.")
	return nil
}

func checksumWorker(jobs <-chan string, results chan<- FileResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for path := range jobs {
		checksum, err := calculateChecksum(path)
		results <- FileResult{Path: path, Checksum: checksum, Err: err}
	}
}

func calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
