package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var pipeSrc string
var pipeDest string

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Ejecuta Extract -> Normalize -> Checksum -> Move",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Iniciando Pipeline de Procesamiento ===")

		tmpDir, err := os.MkdirTemp("", "miapp-workspace-*")
		if err != nil {
			fmt.Printf("Error creando temporal: %v\n", err)
			return
		}
		defer os.RemoveAll(tmpDir)
		fmt.Printf("[1/5] Temporal creado: %s\n", tmpDir)

		fmt.Println("\n[2/5] Extrayendo archivos...")
		if err := RunExtractionLogic(pipeSrc, tmpDir); err != nil {
			fmt.Printf("Fallo en extracción: %v\n", err)
			return
		}

		fmt.Println("\n[3/5] Normalizando nombres...")
		if err := RunNormalizationLogic(tmpDir); err != nil {
			fmt.Printf("Fallo en normalización: %v\n", err)
			return
		}

		fmt.Println("\n[4/5] Calculando Checksums...")
		if err := RunChecksumLogic(tmpDir); err != nil {
			fmt.Printf("Fallo en checksums: %v\n", err)
			return
		}

		fmt.Println("\n[5/5] Moviendo al destino final...")
		if err := os.MkdirAll(pipeDest, os.ModePerm); err != nil {
			fmt.Printf("Error creando destino: %v\n", err)
			return
		}

		movedCount := 0
		err = filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			relPath, _ := filepath.Rel(tmpDir, path)
			finalPath := filepath.Join(pipeDest, relPath)
			os.MkdirAll(filepath.Dir(finalPath), os.ModePerm)

			if err := moveFileRobust(path, finalPath); err != nil {
				fmt.Printf("Error moviendo %s: %v\n", filepath.Base(path), err)
			} else {
				movedCount++
			}
			return nil
		})

		if err != nil {
			fmt.Printf("Error en traslado final: %v\n", err)
		}
		fmt.Printf("\n=== Pipeline completado. %d archivos movidos. ===\n", movedCount)
	},
}

func init() {
	rootCmd.AddCommand(pipelineCmd)
	pipelineCmd.Flags().StringVarP(&pipeSrc, "src", "s", "", "Directorio origen")
	pipelineCmd.Flags().StringVarP(&pipeDest, "dest", "d", "", "Directorio destino final")
	pipelineCmd.MarkFlagRequired("src")
	pipelineCmd.MarkFlagRequired("dest")
}

func moveFileRobust(src, dest string) error {
	err := os.Rename(src, dest)
	if err != nil && strings.Contains(err.Error(), "cross-device link") {
		return copyAndRemove(src, dest)
	}
	return err
}

func copyAndRemove(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	in.Close()
	return os.Remove(src)
}
