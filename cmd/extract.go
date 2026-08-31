package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"asset/internal/extract"

	"github.com/spf13/cobra"
)

func NewExtractCmd(extractor *extract.ExtractorService) *cobra.Command {
	var srcDir, destDir string
	var minFree int64
	var removeSource bool
	var errorDir string
	var password string

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extrae archivos comprimidos masivamente",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return extractor.ExtractAll(ctx, extract.ExtractOptions{
				Src:          srcDir,
				Dest:         destDir,
				Workers:      workers,
				Sync:         syncWrites,
				MinFree:      minFree,
				RemoveSource: removeSource,
				ErrorDir:     resolveErrorDir(destDir, errorDir),
				Password:     password,
			})
		},
	}

	// Los flags se definen localmente para este comando
	cmd.Flags().StringVarP(&srcDir, "src", "s", "", "Directorio origen (requerido)")
	cmd.Flags().StringVarP(&destDir, "dest", "d", "", "Directorio destino (requerido)")
	cmd.Flags().Int64Var(&minFree, "min-free", 1<<30, "Espacio libre mínimo requerido en destino (bytes)")
	cmd.Flags().BoolVar(&removeSource, "remove-source", false, "Borra cada comprimido del origen tras extraerlo con éxito")
	cmd.Flags().StringVar(&errorDir, "error-dir", "", "Directorio de cuarentena para los que fallan (por defecto, .errores junto a dest)")
	cmd.Flags().StringVar(&password, "password", "", "Contraseña para archivos cifrados (RAR y 7z)")

	cmd.MarkFlagRequired("src")
	cmd.MarkFlagRequired("dest")

	return cmd
}
