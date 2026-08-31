package cmd

import (
	"asset/internal/asset"
	"asset/internal/checksum"
	"asset/internal/database"
	"asset/internal/extract"
	"asset/internal/normalize"
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func NewIngestCmd(extractor *extract.ExtractorService, normalizer *normalize.NormalizerService, checksummer *checksum.ChecksumService) *cobra.Command {
	var ingSrc string
	var ingDest string
	var ingDryRun bool
	var ingMinFree int64
	var ingRemoveSource bool
	var ingErrorDir string
	var ingPassword string

	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingiere archivos (procesa y indexa en la base de datos)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngest(cmd.Context(), extractor, normalizer, checksummer, ingSrc, ingDest, dbPath, ingMinFree, ingRemoveSource, resolveErrorDir(ingDest, ingErrorDir), ingPassword, ingDryRun)
		},
	}

	cmd.Flags().StringVarP(&ingSrc, "src", "s", "", "Directorio origen")
	cmd.Flags().StringVarP(&ingDest, "dest", "d", "", "Directorio destino final")
	cmd.Flags().BoolVar(&ingDryRun, "dry-run", false, "Mostrar los movimientos sin aplicarlos")
	cmd.Flags().Int64Var(&ingMinFree, "min-free", 1<<30, "Espacio libre mínimo requerido en destino (bytes)")
	cmd.Flags().BoolVar(&ingRemoveSource, "remove-source", false, "Borra cada comprimido del origen tras extraerlo con éxito")
	cmd.Flags().StringVar(&ingErrorDir, "error-dir", "", "Directorio de cuarentena para los que fallan (por defecto, .errores junto a dest)")
	cmd.Flags().StringVar(&ingPassword, "password", "", "Contraseña para archivos cifrados (RAR y 7z)")
	cmd.MarkFlagRequired("src")
	cmd.MarkFlagRequired("dest")

	return cmd
}

func runIngest(ctx context.Context, extractor *extract.ExtractorService, normalizer *normalize.NormalizerService, checksummer *checksum.ChecksumService, src, dest, dbPath string, minFree int64, removeSource bool, errorDir string, password string, dryRun bool) error {
	if err := runProcess(ctx, extractor, normalizer, checksummer, src, dest, minFree, removeSource, errorDir, password, dryRun); err != nil {
		return err
	}

	if dryRun {
		fmt.Println("\n(dry-run) Se omite el indexado en la base de datos.")
		return nil
	}

	fmt.Println("\n=== Indexando en la base de datos ===")
	db, err := database.InitDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	svc := asset.NewScannerService(asset.NewSQLiteRepo(db))
	if err := svc.ScanDirectory(dest, "local", numWorkers()); err != nil {
		return fmt.Errorf("indexando %s: %w", dest, err)
	}
	return nil
}
