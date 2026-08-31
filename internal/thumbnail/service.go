package thumbnail

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type ThumbnailService struct {
	repo Repository
}

func NewThumbnailService(repo Repository) *ThumbnailService {
	return &ThumbnailService{repo: repo}
}

// ProcessPending busca en la BD y genera miniaturas concurrentemente
func (s *ThumbnailService) ProcessPending(destDir string) error {
	targets, err := s.repo.GetBlobsWithoutThumbnails(1000)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		fmt.Println("No hay miniaturas pendientes por generar.")
		return nil
	}

	fmt.Printf("Generando %d miniaturas...\n", len(targets))

	var wg sync.WaitGroup
	jobs := make(chan TargetBlob, 100)

	// Iniciar 4 Workers (las conversiones de imagen/video consumen mucha CPU)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range jobs {
				s.generateAndSave(target, destDir)
			}
		}()
	}

	for _, t := range targets {
		jobs <- t
	}
	close(jobs)
	wg.Wait()

	return nil
}

func (s *ThumbnailService) generateAndSave(target TargetBlob, destDir string) {
	// Generar nombre de archivo único
	baseName := strings.TrimSuffix(filepath.Base(target.Path), filepath.Ext(target.Path))
	thumbPath := filepath.Join(destDir, ".thumbs", fmt.Sprintf("%s_thumb.jpg", baseName))

	var err error
	if strings.HasPrefix(target.Mime, "video/") {
		// Ejemplo usando FFmpeg local para extraer un frame
		err = exec.Command("ffmpeg", "-y", "-i", target.Path, "-ss", "00:00:02", "-vframes", "1", "-scale", "320:-1", thumbPath).Run()
	} else if strings.HasPrefix(target.Mime, "image/") {
		// Ejemplo usando ImageMagick
		err = exec.Command("magick", target.Path, "-resize", "320x", thumbPath).Run()
	}

	if err == nil {
		// Aquí llamarías a tu función de checksum para la miniatura
		// checksum, size := calculadorChecksum(thumbPath)
		// s.repo.SaveThumbnail(target.ID, thumbPath, size, checksum, 320, 240)
		fmt.Printf("Miniatura generada: %s\n", thumbPath)
	} else {
		fmt.Printf("Error generando miniatura para %s: %v\n", target.Path, err)
	}
}
