package cmd

import (
	"asset/internal/normalize"

	"github.com/spf13/cobra"
)

func NewNormalizeCmd(svc *normalize.NormalizerService) *cobra.Command {
	var normDir string
	var normDryRun bool

	cmd := &cobra.Command{
		Use:   "normalize",
		Short: "Normaliza recursivamente nombres de archivos a slugs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.NormalizeAll(normDir, normDryRun)
		},
	}

	cmd.Flags().StringVarP(&normDir, "dir", "d", "", "Directorio a normalizar (requerido)")
	cmd.Flags().BoolVar(&normDryRun, "dry-run", false, "Mostrar los cambios sin aplicarlos")
	cmd.MarkFlagRequired("dir")

	return cmd
}
