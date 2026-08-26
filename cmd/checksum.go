package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var checkDir string
var checkOutput string

type FileResult struct {
	Path     string
	Checksum string
	Err      error
}

var checksumCmd = &cobra.Command{
	Use:   "checksum",
	Short: "Calcula los checksums SHA-256 masivamente",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunChecksumLogic(checkDir, checkOutput)
	},
}

func init() {
	rootCmd.AddCommand(checksumCmd)
	checksumCmd.Flags().StringVarP(&checkDir, "dir", "d", "", "Directorio a procesar (requerido)")
	checksumCmd.Flags().StringVarP(&checkOutput, "output", "o", "checksums.txt", "Nombre del fichero de checksums")
	checksumCmd.MarkFlagRequired("dir")
}

func RunChecksumLogic(targetDir, outputFile string) error {
	var filesToProcess []string

	err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			filesToProcess = append(filesToProcess, path)
		}
		return nil
	})

	if err != nil {
		return err
	}
	if len(filesToProcess) == 0 {
		fmt.Println("No hay archivos para procesar checksums.")
		return nil
	}

	totalFiles := len(filesToProcess)
	bar := progressbar.NewOptions(totalFiles,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetDescription("Calculando Checksums"),
	)

	jobs := make(chan string, totalFiles)
	results := make(chan FileResult, totalFiles)
	var wg sync.WaitGroup
	var drainer sync.WaitGroup

	nw := numWorkers()
	for w := 1; w <= nw; w++ {
		wg.Add(1)
		go checksumWorker(jobs, results, &wg)
	}

	var checksums []FileResult
	drainer.Add(1)
	go func() {
		defer drainer.Done()
		for res := range results {
			bar.Add(1)
			if res.Err != nil {
				bar.Describe(fmt.Sprintf("[ERROR] %s", filepath.Base(res.Path)))
			} else {
				checksums = append(checksums, res)
			}
		}
	}()

	for _, path := range filesToProcess {
		jobs <- path
	}
	close(jobs)
	wg.Wait()
	close(results)
	drainer.Wait()

	fmt.Println()

	sort.Slice(checksums, func(i, j int) bool {
		return checksums[i].Path < checksums[j].Path
	})

	if len(checksums) > 0 {
		if err := writeChecksumsFile(targetDir, outputFile, checksums); err != nil {
			return err
		}
	}

	fmt.Printf("Cálculo de checksums completado. %d archivos procesados.\n", len(checksums))
	return nil
}

func writeChecksumsFile(targetDir, outputFile string, results []FileResult) error {
	outPath := filepath.Join(targetDir, outputFile)
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("error creando %s: %w", outPath, err)
	}
	defer f.Close()

	for _, res := range results {
		rel, relErr := filepath.Rel(targetDir, res.Path)
		if relErr != nil {
			rel = res.Path
		}
		if _, err := fmt.Fprintf(f, "%s  %s\n", res.Checksum, rel); err != nil {
			return err
		}
	}
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
