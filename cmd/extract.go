package cmd

import (
	"asset/internal/extract"

	"github.com/spf13/cobra"
)

func NewExtractCmd(extractor *extract.ExtractorService) *cobra.Command {
	var srcDir, destDir string
	var minFree int64

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extrae archivos comprimidos masivamente",
		RunE: func(cmd *cobra.Command, args []string) error {
			return extractor.ExtractAll(srcDir, destDir, workers, syncWrites, minFree)
		},
	}

	// Los flags se definen localmente para este comando
	cmd.Flags().StringVarP(&srcDir, "src", "s", "", "Directorio origen (requerido)")
	cmd.Flags().StringVarP(&destDir, "dest", "d", "", "Directorio destino (requerido)")
	cmd.Flags().Int64Var(&minFree, "min-free", 1<<30, "Espacio libre mínimo requerido en destino (bytes)")

	cmd.MarkFlagRequired("src")
	cmd.MarkFlagRequired("dest")

	return cmd
}
