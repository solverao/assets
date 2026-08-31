package cmd

import (
	"asset/internal/asset"
	"asset/internal/database"

	"github.com/spf13/cobra"
)

func NewScanCmd() *cobra.Command {
	var scanDir string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Indexa un árbol de archivos en la base de datos",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := database.InitDB(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			svc := asset.NewScannerService(asset.NewSQLiteRepo(db))
			return svc.ScanDirectory(scanDir, "local", numWorkers())
		},
	}

	cmd.Flags().StringVarP(&scanDir, "dir", "d", "", "Directorio a escanear (requerido)")
	cmd.MarkFlagRequired("dir")

	return cmd
}
