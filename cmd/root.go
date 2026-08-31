package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"asset/internal/checksum"
	"asset/internal/extract"
	"asset/internal/normalize"
	"asset/internal/vault"

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
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Ruta de la base de datos SQLite (por defecto, bóveda actual o ASSET_DB)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			dbPath = resolveDBPath()
		}
		return nil
	}

	rootCmd.AddCommand(NewExtractCmd(extract.NewExtractorService()))
	rootCmd.AddCommand(NewNormalizeCmd(normalize.NewNormalizerService()))
	rootCmd.AddCommand(NewChecksumCmd(checksum.NewChecksumService()))
	rootCmd.AddCommand(NewProcessCmd(extract.NewExtractorService(), normalize.NewNormalizerService(), checksum.NewChecksumService()))
	rootCmd.AddCommand(NewIngestCmd(extract.NewExtractorService(), normalize.NewNormalizerService(), checksum.NewChecksumService()))
	rootCmd.AddCommand(NewScanCmd())
	rootCmd.AddCommand(NewDBCmd())
	rootCmd.AddCommand(NewVaultCmd())
}

// resolveDBPath determina la ruta de la BD: ASSET_DB -> bóveda actual -> assets.db.
func resolveDBPath() string {
	if p := os.Getenv("ASSET_DB"); p != "" {
		return p
	}
	dir, err := vault.ConfigDir()
	if err != nil {
		return "assets.db"
	}
	return resolveDBPathFrom(dir)
}

// resolveDBPathFrom resuelve la BD a partir del registro de bóvedas en dir.
func resolveDBPathFrom(dir string) string {
	if r, err := vault.Load(dir); err == nil && r.Current != "" {
		if p, ok := r.Vaults[r.Current]; ok && p != "" {
			return p
		}
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
