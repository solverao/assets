package cmd

import (
	"asset/internal/checksum"
	"asset/internal/extract"
	"asset/internal/normalize"
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

func NewProcessCmd(extractor *extract.ExtractorService, normalizer *normalize.NormalizerService, checksummer *checksum.ChecksumService) *cobra.Command {
	var procSrc string
	var procDest string
	var procDryRun bool
	var procMinFree int64
	var procRemoveSource bool
	var procErrorDir string

	cmd := &cobra.Command{
		Use:   "process",
		Short: "Procesa archivos (Extract -> Normalize -> Checksum -> Move)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProcess(cmd.Context(), extractor, normalizer, checksummer, procSrc, procDest, procMinFree, procRemoveSource, resolveErrorDir(procDest, procErrorDir), procDryRun)
		},
	}

	cmd.Flags().StringVarP(&procSrc, "src", "s", "", "Directorio origen")
	cmd.Flags().StringVarP(&procDest, "dest", "d", "", "Directorio destino final")
	cmd.Flags().BoolVar(&procDryRun, "dry-run", false, "Mostrar los movimientos sin aplicarlos")
	cmd.Flags().Int64Var(&procMinFree, "min-free", 1<<30, "Espacio libre mínimo requerido en destino (bytes)")
	cmd.Flags().BoolVar(&procRemoveSource, "remove-source", false, "Borra cada comprimido del origen tras extraerlo con éxito")
	cmd.Flags().StringVar(&procErrorDir, "error-dir", "", "Directorio de cuarentena para los que fallan (por defecto, .errores junto a dest)")
	cmd.MarkFlagRequired("src")
	cmd.MarkFlagRequired("dest")

	return cmd
}

func runProcess(ctx context.Context, extractor *extract.ExtractorService, normalizer *normalize.NormalizerService, checksummer *checksum.ChecksumService, src, dest string, minFree int64, removeSource bool, errorDir string, dryRun bool) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("=== Procesamiento de archivos ===")

	// Temporal en el mismo filesystem que dest para evitar copia cross-device.
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("error creando directorio de destino: %w", err)
	}
	tmpDir, err := os.MkdirTemp(destDir, ".asset-tmp-*")
	if err != nil {
		return fmt.Errorf("error creando temporal: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	fmt.Printf("[1/5] Temporal creado: %s\n", tmpDir)

	fmt.Println("\n[2/5] Extrayendo archivos...")
	if err := extractor.ExtractAll(src, tmpDir, numWorkers(), syncWrites, minFree, removeSource, errorDir); err != nil {
		return fmt.Errorf("fallo en extracción: %w", err)
	}

	fmt.Println("\n[3/5] Normalizando nombres...")
	if err := normalizer.NormalizeAll(tmpDir, false); err != nil {
		return fmt.Errorf("fallo en normalización: %w", err)
	}

	fmt.Println("\n[4/5] Calculando Checksums...")
	if err := checksummer.ChecksumAll(tmpDir, "checksums.txt", numWorkers()); err != nil {
		return fmt.Errorf("fallo en checksums: %w", err)
	}

	if dryRun {
		fmt.Println("\n[5/5] Moviendo al destino final (dry-run)...")
		return walkAndMove(ctx, tmpDir, dest, true)
	}

	fmt.Println("\n[5/5] Moviendo al destino final...")
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return fmt.Errorf("error creando destino: %w", err)
	}
	return walkAndMove(ctx, tmpDir, dest, false)
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
	fmt.Printf("\n=== Procesamiento completado. %d archivos movidos. ===\n", movedCount)
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
