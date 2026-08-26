package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

var pipeSrc string
var pipeDest string
var pipeDryRun bool

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Ejecuta Extract -> Normalize -> Checksum -> Move",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPipeline(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(pipelineCmd)
	pipelineCmd.Flags().StringVarP(&pipeSrc, "src", "s", "", "Directorio origen")
	pipelineCmd.Flags().StringVarP(&pipeDest, "dest", "d", "", "Directorio destino final")
	pipelineCmd.Flags().BoolVar(&pipeDryRun, "dry-run", false, "Mostrar los movimientos sin aplicarlos")
	pipelineCmd.MarkFlagRequired("src")
	pipelineCmd.MarkFlagRequired("dest")
}

func runPipeline(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("=== Iniciando Pipeline de Procesamiento ===")

	tmpDir, err := os.MkdirTemp("", "miapp-workspace-*")
	if err != nil {
		return fmt.Errorf("error creando temporal: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	fmt.Printf("[1/5] Temporal creado: %s\n", tmpDir)

	fmt.Println("\n[2/5] Extrayendo archivos...")
	if err := RunExtractionLogic(pipeSrc, tmpDir); err != nil {
		return fmt.Errorf("fallo en extracción: %w", err)
	}

	fmt.Println("\n[3/5] Normalizando nombres...")
	if err := RunNormalizationLogic(tmpDir, false); err != nil {
		return fmt.Errorf("fallo en normalización: %w", err)
	}

	fmt.Println("\n[4/5] Calculando Checksums...")
	if err := RunChecksumLogic(tmpDir, "checksums.txt"); err != nil {
		return fmt.Errorf("fallo en checksums: %w", err)
	}

	if pipeDryRun {
		fmt.Println("\n[5/5] Moviendo al destino final (dry-run)...")
		return walkAndMove(ctx, tmpDir, pipeDest, true)
	}

	fmt.Println("\n[5/5] Moviendo al destino final...")
	if err := os.MkdirAll(pipeDest, os.ModePerm); err != nil {
		return fmt.Errorf("error creando destino: %w", err)
	}
	return walkAndMove(ctx, tmpDir, pipeDest, false)
}

func walkAndMove(ctx context.Context, tmpDir, dest string, dryRun bool) error {
	movedCount := 0
	err := filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(tmpDir, path)
		finalPath := filepath.Join(dest, relPath)

		if dryRun {
			fmt.Printf("Movería %s -> %s\n", path, finalPath)
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(finalPath), os.ModePerm); err != nil {
			return err
		}
		if err := moveFileRobust(path, finalPath); err != nil {
			warnf("Error moviendo %s: %v", filepath.Base(path), err)
			return nil
		}
		movedCount++
		return nil
	})

	if err != nil {
		return fmt.Errorf("error en traslado final: %w", err)
	}
	fmt.Printf("\n=== Pipeline completado. %d archivos movidos. ===\n", movedCount)
	return nil
}

func moveFileRobust(src, dest string) error {
	err := os.Rename(src, dest)
	if err != nil && errors.Is(err, syscall.EXDEV) {
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

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	in.Close()
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
