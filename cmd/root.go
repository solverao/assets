package cmd

import (
	"fmt"
	"os"
	"runtime"

	"asset/internal/checksum"
	"asset/internal/extract"
	"asset/internal/normalize"

	"github.com/spf13/cobra"
)

// version se sobreescribe en build con -ldflags "-X asset/cmd.version=<v>".
var version = "dev"

var (
	verbose    bool
	workers    int
	syncWrites bool
)

var rootCmd = &cobra.Command{
	Use:     "asset",
	Short:   "Herramienta CLI para gestión masiva de archivos",
	Long:    `asset permite extraer, normalizar y procesar archivos masivamente de forma concurrente.`,
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Mostrar información de depuración")
	rootCmd.PersistentFlags().IntVarP(&workers, "workers", "w", 0, "Número de workers concurrentes (0 = auto)")
	rootCmd.PersistentFlags().BoolVar(&syncWrites, "sync", true, "Sincronizar escrituras a disco (fsync) para mayor durabilidad")

	rootCmd.AddCommand(NewExtractCmd(extract.NewExtractorService()))
	rootCmd.AddCommand(NewNormalizeCmd(normalize.NewNormalizerService()))
	rootCmd.AddCommand(NewChecksumCmd(checksum.NewChecksumService()))
	rootCmd.AddCommand(NewPipelineCmd(normalize.NewNormalizerService(), checksum.NewChecksumService()))
	rootCmd.AddCommand(NewScanCmd())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func debugf(format string, a ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", a...)
	}
}

func warnf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "[WARN] "+format+"\n", a...)
}

func numWorkers() int {
	if workers < 1 {
		return runtime.NumCPU()
	}
	return workers
}
