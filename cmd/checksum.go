package cmd

import (
	"asset/internal/checksum"

	"github.com/spf13/cobra"
)

func NewChecksumCmd(svc *checksum.ChecksumService) *cobra.Command {
	var checkDir string
	var checkOutput string

	cmd := &cobra.Command{
		Use:   "checksum",
		Short: "Calcula los checksums SHA-256 masivamente",
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.ChecksumAll(checkDir, checkOutput, numWorkers())
		},
	}

	cmd.Flags().StringVarP(&checkDir, "dir", "d", "", "Directorio a procesar (requerido)")
	cmd.Flags().StringVarP(&checkOutput, "output", "o", "checksums.txt", "Nombre del fichero de checksums")
	cmd.MarkFlagRequired("dir")

	return cmd
}
