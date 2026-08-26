package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "asset",
	Short: "Herramienta CLI para gestión masiva de archivos",
	Long:  `asset permite extraer, normalizar y procesar archivos masivamente de forma concurrente.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
