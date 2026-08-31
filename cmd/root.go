package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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
	dbPath     string
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
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath(), "Ruta de la base de datos SQLite")

	rootCmd.AddCommand(NewExtractCmd(extract.NewExtractorService()))
	rootCmd.AddCommand(NewNormalizeCmd(normalize.NewNormalizerService()))
	rootCmd.AddCommand(NewChecksumCmd(checksum.NewChecksumService()))
	rootCmd.AddCommand(NewProcessCmd(extract.NewExtractorService(), normalize.NewNormalizerService(), checksum.NewChecksumService()))
	rootCmd.AddCommand(NewIngestCmd(extract.NewExtractorService(), normalize.NewNormalizerService(), checksum.NewChecksumService()))
	rootCmd.AddCommand(NewScanCmd())
	rootCmd.AddCommand(NewDBCmd())
}

func defaultDBPath() string {
	if p := os.Getenv("ASSET_DB"); p != "" {
		return p
	}
	return "assets.db"
}

// resolveErrorDir devuelve el directorio de cuarentena de errores. Si flagVal
// está vacío, usa un directorio oculto .errores junto a dest.
func resolveErrorDir(dest, flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return filepath.Join(filepath.Dir(dest), ".errores")
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
